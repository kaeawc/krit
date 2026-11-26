#!/usr/bin/env python3
"""Generate the compose-lints library tables from the jars on the Lint parity classpath.

usage: python3 -I scripts/compose-lints/gen_tables.py JAR...

Writes internal/rules/compose_lints_composable_tables.go (which library functions and getters are
@Composable / @ReadOnlyComposable, and which packages the jars declare) and
internal/rules/compose_lints_stability_tables.go (compose-lints' isStable verdict for public Compose types named
*Scope, *State or *Modifier). Pass the same jars the Lint CLI ran with when recording testdata/compose_lints
(Compose 1.10.0, material3 1.4.0, ui-tooling-preview, Wear compose-ui-tooling 1.5.0, Glance 1.1.1,
lifecycle-viewmodel, kotlin-stdlib, kotlinx-coroutines-core, kotlinx-collections-immutable). Run gofmt after.
Only the standard library is used; class files are read directly.
"""
import os
import re
import sys
import struct, sys, zipfile, json

def parse(data):
    pos = 8
    (count,) = struct.unpack_from('>H', data, pos); pos += 2
    cp = [None] * count
    i = 1
    while i < count:
        tag = data[pos]; pos += 1
        if tag == 1:
            (ln,) = struct.unpack_from('>H', data, pos); pos += 2
            cp[i] = ('utf8', data[pos:pos + ln].decode('utf-8', 'replace')); pos += ln
        elif tag in (3, 4):
            pos += 4
        elif tag in (5, 6):
            pos += 8; i += 1
        elif tag == 7:
            (idx,) = struct.unpack_from('>H', data, pos); pos += 2; cp[i] = ('class', idx)
        elif tag in (8, 16, 19, 20):
            pos += 2
        elif tag in (9, 10, 11, 12, 17, 18):
            pos += 4
        elif tag == 15:
            pos += 3
        else:
            raise ValueError('tag %d' % tag)
        i += 1
    utf = lambda k: cp[k][1]
    cls = lambda k: utf(cp[k][1]) if k else None
    flags, this, sup = struct.unpack_from('>HHH', data, pos); pos += 6
    (ni,) = struct.unpack_from('>H', data, pos); pos += 2
    ifaces = [cls(struct.unpack_from('>H', data, pos + 2 * j)[0]) for j in range(ni)]; pos += 2 * ni
    methods = []
    for kind in range(2):  # fields, methods
        (n,) = struct.unpack_from('>H', data, pos); pos += 2
        for _ in range(n):
            mflags, mname, _desc = struct.unpack_from('>HHH', data, pos); pos += 6
            (na,) = struct.unpack_from('>H', data, pos); pos += 2
            manns = []
            for _ in range(na):
                (ani, alen) = struct.unpack_from('>HI', data, pos)
                if kind == 1 and utf(ani) in ('RuntimeInvisibleAnnotations', 'RuntimeVisibleAnnotations'):
                    (nann,) = struct.unpack_from('>H', data, pos + 6)
                    p = pos + 8
                    for _ in range(nann):
                        (ti,) = struct.unpack_from('>H', data, p)
                        manns.append(utf(ti))
                        p = skip_annotation(data, p)
                pos += 6 + alen
            if kind == 1:
                methods.append({'name': utf(mname), 'flags': mflags, 'annotations': manns})
    (na,) = struct.unpack_from('>H', data, pos); pos += 2
    anns = []
    for _ in range(na):
        (ni2, alen) = struct.unpack_from('>HI', data, pos); pos += 6
        if utf(ni2) in ('RuntimeInvisibleAnnotations', 'RuntimeVisibleAnnotations'):
            (nann,) = struct.unpack_from('>H', data, pos)
            p = pos + 2
            for _ in range(nann):
                (ti,) = struct.unpack_from('>H', data, p)
                anns.append(utf(ti))
                p = skip_annotation(data, p)
        pos += alen
    return {'name': cls(this), 'flags': flags, 'super': cls(sup), 'interfaces': ifaces, 'annotations': anns, 'methods': methods}

def skip_annotation(data, p):
    p += 2
    (npairs,) = struct.unpack_from('>H', data, p); p += 2
    for _ in range(npairs):
        p += 2
        p = skip_value(data, p)
    return p

def skip_value(data, p):
    tag = chr(data[p]); p += 1
    if tag in 'BCDFIJSZsc':
        return p + 2
    if tag == 'e':
        return p + 4
    if tag == '@':
        return skip_annotation(data, p)
    if tag == '[':
        (n,) = struct.unpack_from('>H', data, p); p += 2
        for _ in range(n):
            p = skip_value(data, p)
        return p
    raise ValueError(tag)



def load(jars):
    out = {}
    for jar in jars:
        z = zipfile.ZipFile(jar)
        for n in z.namelist():
            if n.endswith('.class') and not n.endswith('module-info.class'):
                try:
                    info = parse(z.read(n))
                except Exception:
                    continue
                out[info['name']] = info
    return out


COMPOSABLE = 'Landroidx/compose/runtime/Composable;'
READ_ONLY = 'Landroidx/compose/runtime/ReadOnlyComposable;'
STABILITY = {'Landroidx/compose/runtime/Stable;', 'Landroidx/compose/runtime/Immutable;'}
KNOWN_STABLE = {'kotlin/Pair', 'kotlin/Triple', 'java/util/Comparator', 'kotlin/Result', 'kotlin/ranges/ClosedRange',
                'kotlin/ranges/ClosedFloatingPointRange', 'java/lang/String'}
EXTRA_UNSTABLE = ['android.app.Activity', 'android.app.Application', 'android.app.Service', 'android.content.Context',
                  'android.view.View', 'androidx.activity.ComponentActivity',
                  'androidx.appcompat.app.AppCompatActivity', 'androidx.fragment.app.Fragment',
                  'androidx.fragment.app.FragmentActivity', 'androidx.lifecycle.ViewModel',
                  'androidx.lifecycle.AndroidViewModel', 'androidx.glance.appwidget.GlanceAppWidget',
                  'androidx.glance.appwidget.GlanceAppWidgetReceiver']


def plain(name):
    return re.sub(r'-[A-Za-z0-9_]+$', '', name)  # strip inline-class mangling


def composables(classes):
    top, members, by_class, pkgs = {}, {}, {}, set()
    for n, c in classes.items():
        pkg = n.rsplit('/', 1)[0].replace('/', '.') if '/' in n else ''
        if pkg:
            pkgs.add(pkg)
        base = n.rsplit('/', 1)[-1]
        for m in c['methods']:
            if COMPOSABLE not in m['annotations']:
                continue
            kind = 2 if READ_ONLY in m['annotations'] else 1
            name = plain(m['name'])
            if name.startswith('access$') or name in ('<init>', '<clinit>'):
                continue
            names = [name]
            if re.match(r'get[A-Z]', name):
                names.append(name[3].lower() + name[4:])
            if base.endswith('Kt') and m['flags'] & 0x0008 and '$' not in base:
                for nm in names:
                    k = pkg + '.' + nm
                    top[k] = min(top.get(k, kind), kind)
            else:
                for nm in names:
                    members[nm] = min(members.get(nm, kind), kind)
                if not base.endswith('Kt'):
                    for nm in names:
                        k = n.replace('/', '.').replace('$', '.') + '.' + nm
                        by_class[k] = min(by_class.get(k, kind), kind)
    return top, members, by_class, pkgs


def stability(classes):
    memo = {}

    def stable(n, seen=()):
        if n in memo:
            return memo[n]
        if n == 'java/lang/Object':
            return False
        c = classes.get(n)
        if c is None:
            return None
        if n in seen:
            return False
        anns = set(c['annotations'])
        verdict = None
        if (c['flags'] & 0x4000 or n in KNOWN_STABLE or n.startswith('kotlin/jvm/functions/') or
                n.startswith('kotlin/Function') or anns & STABILITY or 'Ljava/lang/FunctionalInterface;' in anns or
                'Lkotlin/jvm/JvmInline;' in anns):
            verdict = True
        for a in anns:
            ac = classes.get(a[1:-1])
            if ac and 'Landroidx/compose/runtime/StableMarker;' in ac['annotations']:
                verdict = True
        if verdict is None:
            unknown = False
            for s in [c['super']] + c['interfaces']:
                if not s:
                    continue
                v = stable(s, seen + (n,))
                if v:
                    verdict = True
                    break
                if v is None:
                    unknown = True
            if verdict is None:
                verdict = None if unknown else False
        memo[n] = verdict
        return verdict

    st, un = set(), set()
    for n, c in classes.items():
        base = n.rsplit('/', 1)[-1]
        if (not n.startswith('androidx/compose/') or '$' in base or base.endswith('Kt') or
                not re.search(r'(Scope|State|Modifier)$', base) or not c['flags'] & 0x0001):
            continue
        v = stable(n)
        if v is True:
            st.add(n.replace('/', '.'))
        elif v is False:
            un.add(n.replace('/', '.'))
    return st, un


def fmt_map(m):
    return fmt([('"%s": %d, ' % (k, m[k])) for k in sorted(m)])


def fmt_set(s):
    return fmt([('"%s": true, ' % k) for k in sorted(s)])


def fmt(items):
    lines, cur = [], '\t'
    for item in items:
        if len(cur) + len(item) > 118:
            lines.append(cur.rstrip())
            cur = '\t'
        cur += item
    lines.append(cur.rstrip())
    return '\n'.join(lines) + '\n'


HEADER = '// Code generated by scripts/compose-lints/gen_tables.py; DO NOT EDIT.\n\n'


def main():
    classes = load(sys.argv[1:])
    root = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', '..', 'internal', 'rules')
    top, members, by_class, pkgs = composables(classes)
    with open(os.path.join(root, 'compose_lints_composable_tables.go'), 'w') as f:
        f.write(HEADER + 'package rules\n\n'
                '// Composable library declarations from the compose-lints parity classpath. Values: 1 @Composable,\n'
                '// 2 @Composable @ReadOnlyComposable (a name with any non-read-only overload is 1). Property getters are\n'
                '// also listed under the property name. ComposeRedundantComposable and ComposeReadOnlyComposable classify\n'
                '// library calls with them and abstain on names from packages outside composeLintsScannedPackages.\n\n'
                '// composeLintsLibraryComposableFunctions maps top-level functions and properties by FQN.\n'
                'var composeLintsLibraryComposableFunctions = map[string]int8{\n' + fmt_map(top) + '}\n\n'
                '// composeLintsLibraryComposableMembers maps member functions and properties by simple name.\n'
                'var composeLintsLibraryComposableMembers = map[string]int8{\n' + fmt_map(members) + '}\n\n'
                '// composeLintsLibraryComposableMembersByClass maps members by declaring class FQN and name, for\n'
                '// receivers that resolve to a library class or object (MaterialTheme.shapes).\n'
                'var composeLintsLibraryComposableMembersByClass = map[string]int8{\n' + fmt_map(by_class) + '}\n\n'
                '// composeLintsScannedPackages are the packages those jars declare.\n'
                'var composeLintsScannedPackages = map[string]bool{\n' + fmt_set(pkgs) + '}\n')
    st, un = stability(classes)
    with open(os.path.join(root, 'compose_lints_stability_tables.go'), 'w') as f:
        f.write(HEADER + 'package rules\n\n'
                '// Stability of Compose library types, as compose-lints\' isStable classifies them (an enum, @Stable or\n'
                '// @Immutable on the type or a supertype, a functional interface, a value class), for the public types\n'
                '// named *Scope, *State or *Modifier, the usual composable receivers. ComposeUnstableReceiver uses them\n'
                '// for receivers and supertypes it cannot resolve to project sources; other library types are not judged.\n'
                'var composeLintsStableLibraryTypes = map[string]bool{\n' + fmt_set(st) + '}\n\n'
                'var composeLintsUnstableLibraryTypes = map[string]bool{\n' + fmt_set(un) +
                '\t// Android and AndroidX base classes: no Compose stability annotation on them or their supertypes.\n' +
                fmt_set(EXTRA_UNSTABLE) + '}\n')


if __name__ == '__main__':
    main()
