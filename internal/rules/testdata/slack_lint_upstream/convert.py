"""Converts slack-lints MockDetector unit tests into krit parity case directories.

usage: convert.py <slack-lints mocking test dir> <out testdata dir> <ids,comma> <TestFile.kt>...
Each @Test method becomes <out>/mock_<testclass>_<method>/ with sources, checks.txt and upstream-0.11.1.txt.
"""
import os
import re
import sys
import textwrap


def trim_indent(s):
    lines = s.split("\n")
    if lines and lines[0].strip() == "":
        lines = lines[1:]
    if lines and lines[-1].strip() == "":
        lines = lines[:-1]
    return textwrap.dedent("\n".join(lines))


FILE_RE = re.compile(r'\b(kotlin|java)\(\s*(?:"([^"]*)"\s*,\s*)?"""(.*?)"""', re.S)


def parse_files(text):
    """Returns (start, end, lang, path, content) for each kotlin()/java() test file in text."""
    out = []
    for m in FILE_RE.finditer(text):
        out.append((m.start(), m.end(), m.group(1), m.group(2), trim_indent(m.group(3))))
    return out


def named_values(text):
    """Maps `val NAME =` declarations to the test files they define, in order."""
    files = parse_files(text)
    vals = {}
    for vm in re.finditer(r'\bval\s+(\w+)\s*=', text):
        # the value runs until the next `val`/`fun`/`@Test`, or the end
        nxt = re.compile(r'\n\s*(private\s+|override\s+)?(val|fun)\s|\n\s*@Test').search(text, vm.end())
        end = nxt.start() if nxt else len(text)
        owned = [f for f in files if vm.end() <= f[0] < end]
        if owned:
            vals[vm.group(1)] = owned
            continue
        # `val all = arrayOf(a, b)` refers to other values.
        am = re.match(r'\s*arrayOf\(([^)]*)\)', text[vm.end():end])
        if am:
            body = re.sub(r'//[^\n]*', '', am.group(1))
            refs = [r.strip() for r in body.split(",") if r.strip()]
            if refs and all(r in vals for r in refs):
                vals[vm.group(1)] = [f for r in refs for f in vals[r]]
    return vals


def default_path(lang, content):
    pkg = re.search(r'^\s*package\s+([\w.]+)', content, re.M)
    pkg = pkg.group(1) if pkg else ""
    cls = re.search(r'\b(?:class|interface|object|enum|record|@interface)\s+(\w+)', content)
    name = cls.group(1) if cls else "test"
    ext = "kt" if lang == "kotlin" else "java"
    return "src/main/java/" + (pkg.replace(".", "/") + "/" if pkg else "") + name + "." + ext


def krit_path(lang, path, content):
    if not path:
        return default_path(lang, content)
    if path.startswith("test/"):
        return "src/test/java/" + path[len("test/"):]
    if path.startswith("src/"):
        return path
    return "src/main/java/" + path


def parse_expected(block, path_map, ids):
    lines = trim_indent(block).split("\n")
    out = []
    head = re.compile(r'^(\S+?):(\d+): (Error|Warning|Information): (.*) \[(\w+)\]$')
    for i, line in enumerate(lines):
        m = head.match(line)
        if not m or m.group(5) not in ids:
            continue
        marker = lines[i + 2] if i + 2 < len(lines) else ""
        col = min([p for p in (marker.find("~"), marker.find("^")) if p >= 0], default=-1) + 1
        if col <= 0:
            raise SystemExit("no marker for: " + line)
        out.append(f"{path_map.get(m.group(1), m.group(1))}:{m.group(2)}:{col}: {m.group(5)}: {m.group(4)}")
    return sorted(out)


def main():
    test_dir, out_dir, ids = sys.argv[1], sys.argv[2], set(sys.argv[3].split(","))
    stubs_text = open(os.path.join(test_dir, "MockFileStubs.kt")).read()
    stub_vals = named_values(stubs_text)
    stub_files = [f for name in ("mockito", "mock", "spy", "mockitoHelpers") for f in stub_vals[name]]
    for test_file in sys.argv[4:]:
        text = open(os.path.join(test_dir, test_file)).read()
        cls = re.search(r'class (\w+)Test\b', text).group(1)
        vals = named_values(text)
        for tm in re.finditer(r'@Test\s+fun\s+(\w+)\(\)', text):
            method = tm.group(1)
            nxt = text.find("@Test", tm.end())
            body = text[tm.end(): nxt if nxt >= 0 else len(text)]
            local_vals = named_values(body)
            fm = re.search(r'\.files\((.*?)\)\s*\n', body, re.S)
            em = re.search(r'\.expect\(\s*"""(.*?)"""', body, re.S)
            if not fm or not em:
                continue
            files = []
            for ref in [r.strip() for r in fm.group(1).split(",") if r.strip()]:
                ref = ref.lstrip("*")
                if ref == "mockFileStubs()":
                    files += stub_files
                elif ref.endswith("()"):
                    name = ref[:-2]
                    files += vals.get(name, [])
                else:
                    files += local_vals.get(ref) or vals.get(ref) or []
            case = os.path.join(out_dir, f"mock_{cls[0].lower() + cls[1:]}_{method}")
            path_map = {}
            for (_, _, lang, path, content) in files:
                kp = krit_path(lang, path, content)
                if path:
                    path_map[path] = kp
                else:
                    path_map[kp] = kp
                os.makedirs(os.path.join(case, os.path.dirname(kp)), exist_ok=True)
                with open(os.path.join(case, kp), "w") as fh:
                    fh.write(content + "\n")
            expected = parse_expected(em.group(1), path_map, ids)
            with open(os.path.join(case, "checks.txt"), "w") as fh:
                fh.write(",".join(sorted(ids)) + "\n")
            with open(os.path.join(case, "upstream-0.11.1.txt"), "w") as fh:
                fh.write("\n".join(expected) + ("\n" if expected else ""))
            print(case, len(expected))


main()
