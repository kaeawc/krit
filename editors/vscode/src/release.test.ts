import * as assert from 'node:assert/strict';
import * as crypto from 'node:crypto';
import * as net from 'node:net';
import { test } from 'node:test';
import * as zlib from 'node:zlib';
import { archiveName, detectPlatform, download, expectedChecksum, extractFile, verifyChecksum } from './release';

test('archive names match goreleaser naming', () => {
    assert.equal(archiveName('0.2.0', { os: 'darwin', arch: 'arm64', musl: false }), 'krit_0.2.0_darwin_arm64.tar.gz');
    assert.equal(archiveName('v0.2.0', { os: 'linux', arch: 'amd64', musl: false }), 'krit_0.2.0_linux_amd64.tar.gz');
    assert.equal(archiveName('v0.2.0', { os: 'linux', arch: 'amd64', musl: true }), 'krit_0.2.0_linux_musl_amd64.tar.gz');
    assert.equal(archiveName('v0.2.0', { os: 'windows', arch: 'amd64', musl: false }), 'krit_0.2.0_windows_amd64.zip');
    assert.equal(
        archiveName('v0.3.0-nightly.20260925', { os: 'linux', arch: 'arm64', musl: false }),
        'krit_0.3.0-nightly.20260925_linux_arm64.tar.gz',
    );
});

test('platform detection maps node platforms to release targets', () => {
    const glibc = () => false;
    const musl = () => true;
    assert.deepEqual(detectPlatform('darwin', 'arm64', glibc), { os: 'darwin', arch: 'arm64', musl: false });
    assert.deepEqual(detectPlatform('win32', 'x64', glibc), { os: 'windows', arch: 'amd64', musl: false });
    assert.deepEqual(detectPlatform('linux', 'x64', musl), { os: 'linux', arch: 'amd64', musl: true });
    // No linux/musl/arm64 build: falls back to glibc.
    assert.deepEqual(detectPlatform('linux', 'arm64', musl), { os: 'linux', arch: 'arm64', musl: false });
    assert.throws(() => detectPlatform('win32', 'arm64', glibc), /windows\/arm64/);
    assert.throws(() => detectPlatform('linux', 'ia32', glibc), /Unsupported architecture/);
    assert.throws(() => detectPlatform('freebsd', 'x64', glibc), /Unsupported platform/);
});

test('checksum lookup matches exact archive names', () => {
    const data = Buffer.from('archive-bytes');
    const sha = crypto.createHash('sha256').update(data).digest('hex');
    const sums = [
        `${'0'.repeat(64)}  krit_0.2.0_linux_musl_amd64.tar.gz`,
        `${sha}  krit_0.2.0_linux_amd64.tar.gz`,
        '',
    ].join('\n');
    assert.equal(expectedChecksum(sums, 'krit_0.2.0_linux_amd64.tar.gz'), sha);
    verifyChecksum(data, sums, 'krit_0.2.0_linux_amd64.tar.gz');
    assert.throws(() => verifyChecksum(data, sums, 'krit_0.2.0_linux_musl_amd64.tar.gz'), /Checksum mismatch/);
    assert.throws(() => verifyChecksum(data, sums, 'krit_0.2.0_darwin_arm64.tar.gz'), /not listed/);
});

function tarGz(entries: Array<[string, string, string?]>): Buffer {
    const blocks: Buffer[] = [];
    for (const [name, content, type = '0'] of entries) {
        const header = Buffer.alloc(512);
        header.write(name, 0);
        header.write('0000755\0', 100);
        header.write(`${content.length.toString(8).padStart(11, '0')}\0`, 124);
        header.write(type, 156);
        header.write('ustar\u000000', 257);
        const data = Buffer.from(content);
        blocks.push(header, data, Buffer.alloc((512 - (data.length % 512)) % 512));
    }
    blocks.push(Buffer.alloc(1024));
    return zlib.gzipSync(Buffer.concat(blocks));
}

test('extracts krit-lsp from a tar.gz archive, skipping PAX headers', () => {
    const archive = tarGz([
        ['README.md', 'readme'],
        ['PaxHeaders.0/krit-lsp', '30 mtime=1747000000.0\n', 'x'],
        ['krit', 'krit'],
        ['krit-lsp', 'lsp-binary'],
    ]);
    assert.equal(extractFile(archive, 'krit_0.2.0_linux_amd64.tar.gz', 'krit-lsp').toString(), 'lsp-binary');
    assert.throws(() => extractFile(archive, 'krit_0.2.0_linux_amd64.tar.gz', 'krit-mcp'), /not found/);
});

function zip(entries: Array<[string, string, boolean]>): Buffer {
    const locals: Buffer[] = [];
    const central: Buffer[] = [];
    let offset = 0;
    for (const [name, content, deflate] of entries) {
        const raw = Buffer.from(content);
        const data = deflate ? zlib.deflateRawSync(raw) : raw;
        const nameBuf = Buffer.from(name);
        const local = Buffer.alloc(30);
        local.writeUInt32LE(0x04034b50, 0);
        local.writeUInt16LE(deflate ? 8 : 0, 8);
        local.writeUInt32LE(data.length, 18);
        local.writeUInt32LE(raw.length, 22);
        local.writeUInt16LE(nameBuf.length, 26);
        const cd = Buffer.alloc(46);
        cd.writeUInt32LE(0x02014b50, 0);
        cd.writeUInt16LE(deflate ? 8 : 0, 10);
        cd.writeUInt32LE(data.length, 20);
        cd.writeUInt32LE(raw.length, 24);
        cd.writeUInt16LE(nameBuf.length, 28);
        cd.writeUInt32LE(offset, 42);
        locals.push(local, nameBuf, data);
        central.push(cd, nameBuf);
        offset += local.length + nameBuf.length + data.length;
    }
    const cdBuf = Buffer.concat(central);
    const eocd = Buffer.alloc(22);
    eocd.writeUInt32LE(0x06054b50, 0);
    eocd.writeUInt16LE(entries.length, 8);
    eocd.writeUInt16LE(entries.length, 10);
    eocd.writeUInt32LE(cdBuf.length, 12);
    eocd.writeUInt32LE(offset, 16);
    return Buffer.concat([...locals, cdBuf, eocd]);
}

test('extracts krit-lsp.exe from a windows zip archive', () => {
    const archive = zip([
        ['LICENSE', 'mit', false],
        ['krit.exe', 'krit', true],
        ['krit-lsp.exe', 'lsp-exe'.repeat(100), true],
    ]);
    assert.equal(extractFile(archive, 'krit_0.2.0_windows_amd64.zip', 'krit-lsp.exe').toString(), 'lsp-exe'.repeat(100));
    assert.equal(extractFile(archive, 'krit_0.2.0_windows_amd64.zip', 'LICENSE').toString(), 'mit');
});

test('download rejects when the connection stalls', async () => {
    // Accepts TCP connections but never answers the TLS handshake.
    const sockets: net.Socket[] = [];
    const server = net.createServer((socket) => sockets.push(socket));
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
    const { port } = server.address() as net.AddressInfo;
    try {
        await assert.rejects(download(`https://127.0.0.1:${port}/archive`, {}, 0, 200), /timed out after 200ms/);
    } finally {
        sockets.forEach((s) => s.destroy());
        server.close();
    }
});
