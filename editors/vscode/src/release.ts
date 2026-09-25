// Helpers for locating, verifying, and unpacking krit release archives.
// Kept free of the `vscode` module so they can be unit tested with plain Node.
//
// Release assets follow `.goreleaser.yml`: `krit_<version>_<os>_<arch>.tar.gz`
// (`.zip` on Windows, `krit_<version>_linux_musl_amd64.tar.gz` for musl libc),
// each bundling `krit`, `krit-lsp`, and `krit-mcp` at the archive root, plus a
// `checksums.txt` covering every archive.

import * as crypto from 'crypto';
import * as https from 'https';
import * as zlib from 'zlib';

export const releasesUrl = 'https://github.com/kaeawc/krit/releases';

export interface Platform {
    os: 'darwin' | 'linux' | 'windows';
    arch: 'amd64' | 'arm64';
    musl: boolean;
}

export function detectPlatform(
    platform: NodeJS.Platform = process.platform,
    arch: string = process.arch,
    isMusl: () => boolean = detectMusl,
): Platform {
    const os = platform === 'darwin' ? 'darwin'
        : platform === 'win32' ? 'windows'
        : platform === 'linux' ? 'linux'
        : undefined;
    if (!os) {
        throw new Error(`Unsupported platform: ${platform}`);
    }
    const goarch = arch === 'arm64' ? 'arm64' : arch === 'x64' ? 'amd64' : undefined;
    if (!goarch) {
        throw new Error(`Unsupported architecture: ${arch}`);
    }
    if (os === 'windows' && goarch === 'arm64') {
        throw new Error('krit does not publish a windows/arm64 build; set krit.binaryPath to a locally built krit-lsp');
    }
    // Only linux/amd64 has a musl build; musl on arm64 falls back to the
    // glibc archive, matching install.sh.
    const musl = os === 'linux' && goarch === 'amd64' && isMusl();
    return { os, arch: goarch, musl };
}

function detectMusl(): boolean {
    // Node records the glibc version it runs against; it is absent under musl.
    const report = process.report?.getReport() as { header?: { glibcVersionRuntime?: string } } | undefined;
    return report?.header?.glibcVersionRuntime === undefined;
}

/** Accepts `0.2.0` or `v0.2.0`; tags carry the `v`, archive names do not. */
export function normalizeVersion(version: string): string {
    return version.trim().replace(/^v/, '');
}

export function archiveName(version: string, p: Platform): string {
    const libc = p.musl ? 'musl_' : '';
    const ext = p.os === 'windows' ? 'zip' : 'tar.gz';
    return `krit_${normalizeVersion(version)}_${p.os}_${libc}${p.arch}.${ext}`;
}

export function lspBinaryName(p: Platform): string {
    return p.os === 'windows' ? 'krit-lsp.exe' : 'krit-lsp';
}

/** Expected SHA-256 for `name` in a goreleaser `checksums.txt`, or undefined. */
export function expectedChecksum(checksumsText: string, name: string): string | undefined {
    for (const line of checksumsText.split(/\r?\n/)) {
        const parts = line.trim().split(/\s+/);
        if (parts.length === 2 && parts[1].replace(/^\*/, '') === name) {
            return parts[0].toLowerCase();
        }
    }
    return undefined;
}

/** Throws unless `data` matches the entry for `name` in `checksumsText`. */
export function verifyChecksum(data: Buffer, checksumsText: string, name: string): void {
    const expected = expectedChecksum(checksumsText, name);
    if (!expected) {
        throw new Error(`${name} is not listed in checksums.txt`);
    }
    const actual = crypto.createHash('sha256').update(data).digest('hex');
    if (actual !== expected) {
        throw new Error(`Checksum mismatch for ${name}: expected ${expected}, got ${actual}`);
    }
}

/** Returns the contents of the file whose basename is `fileName`, chosen by archive extension. */
export function extractFile(archive: Buffer, archiveFileName: string, fileName: string): Buffer {
    let contents: Buffer | undefined;
    if (archiveFileName.endsWith('.zip')) {
        contents = readZipEntry(archive, fileName);
    } else if (archiveFileName.endsWith('.tar.gz')) {
        contents = readTarEntry(zlib.gunzipSync(archive), fileName);
    } else {
        throw new Error(`Unsupported archive format: ${archiveFileName}`);
    }
    if (!contents) {
        throw new Error(`${fileName} not found in ${archiveFileName}`);
    }
    return contents;
}

const basename = (name: string) => name.slice(name.lastIndexOf('/') + 1);

function readTarEntry(tar: Buffer, fileName: string): Buffer | undefined {
    // 512-byte header blocks, each followed by file data padded to 512.
    let offset = 0;
    while (offset + 512 <= tar.length) {
        const header = tar.subarray(offset, offset + 512);
        const nameEnd = header.indexOf(0);
        const name = header.toString('utf8', 0, nameEnd === -1 || nameEnd > 100 ? 100 : nameEnd);
        if (name === '') {
            return undefined; // empty header = end of archive
        }
        const size = parseInt(header.toString('ascii', 124, 136).replace(/[\0 ]/g, ''), 8);
        if (Number.isNaN(size)) {
            return undefined;
        }
        // Only regular files ('0' or NUL); skips PAX/GNU metadata headers
        // whose names can share the binary's basename.
        const type = header[156];
        const dataOffset = offset + 512;
        if ((type === 0x30 || type === 0) && basename(name) === fileName) {
            return tar.subarray(dataOffset, dataOffset + size);
        }
        offset = dataOffset + Math.ceil(size / 512) * 512;
    }
    return undefined;
}

function readZipEntry(zip: Buffer, fileName: string): Buffer | undefined {
    // Locate the end-of-central-directory record (it may be followed by a comment).
    let eocd = -1;
    for (let i = zip.length - 22; i >= Math.max(0, zip.length - 22 - 0xffff); i--) {
        if (zip.readUInt32LE(i) === 0x06054b50) {
            eocd = i;
            break;
        }
    }
    if (eocd < 0) {
        throw new Error('Invalid zip archive: no end of central directory');
    }
    const entries = zip.readUInt16LE(eocd + 10);
    let offset = zip.readUInt32LE(eocd + 16);
    for (let n = 0; n < entries; n++) {
        if (zip.readUInt32LE(offset) !== 0x02014b50) {
            throw new Error('Invalid zip archive: bad central directory entry');
        }
        const method = zip.readUInt16LE(offset + 10);
        const compressedSize = zip.readUInt32LE(offset + 20);
        const nameLength = zip.readUInt16LE(offset + 28);
        const extraLength = zip.readUInt16LE(offset + 30);
        const commentLength = zip.readUInt16LE(offset + 32);
        const localOffset = zip.readUInt32LE(offset + 42);
        const name = zip.toString('utf8', offset + 46, offset + 46 + nameLength);
        offset += 46 + nameLength + extraLength + commentLength;

        if (name.endsWith('/') || basename(name) !== fileName) {
            continue;
        }
        const dataOffset = localOffset + 30
            + zip.readUInt16LE(localOffset + 26)
            + zip.readUInt16LE(localOffset + 28);
        const data = zip.subarray(dataOffset, dataOffset + compressedSize);
        if (method === 0) {
            return data;
        }
        if (method === 8) {
            return zlib.inflateRawSync(data);
        }
        throw new Error(`Unsupported zip compression method ${method} for ${name}`);
    }
    return undefined;
}

/** Maps the `krit.version` setting (`latest`, `0.2.0`, or `v0.2.0`) to a release tag. */
export async function resolveReleaseTag(version: string): Promise<string> {
    if (version && version !== 'latest') {
        return `v${normalizeVersion(version)}`;
    }
    const body = await download('https://api.github.com/repos/kaeawc/krit/releases/latest', {
        Accept: 'application/vnd.github+json',
    });
    const tag = (JSON.parse(body.toString('utf-8')) as { tag_name?: string }).tag_name;
    if (!tag) {
        throw new Error('Could not determine the latest krit release');
    }
    return tag;
}

/**
 * GETs `url`, following redirects. `timeoutMs` bounds socket inactivity both
 * while connecting and between chunks, so a stalled connection rejects
 * instead of leaving the download progress notification up forever.
 */
export function download(
    url: string,
    headers: Record<string, string> = {},
    redirects = 5,
    timeoutMs = 30_000,
): Promise<Buffer> {
    return new Promise((resolve, reject) => {
        const options = { headers: { 'User-Agent': 'krit-vscode', ...headers }, timeout: timeoutMs };
        const request = https.get(url, options, (response) => {
            const status = response.statusCode ?? 0;
            if (status >= 300 && status < 400) {
                response.resume();
                const location = response.headers.location;
                if (!location || redirects === 0) {
                    reject(new Error(`Download failed: bad redirect from ${url}`));
                    return;
                }
                download(new URL(location, url).toString(), headers, redirects - 1, timeoutMs).then(resolve, reject);
                return;
            }
            if (status !== 200) {
                response.resume();
                reject(new Error(`Download failed: HTTP ${status} for ${url}`));
                return;
            }
            const chunks: Buffer[] = [];
            response.on('data', (chunk: Buffer) => chunks.push(chunk));
            response.on('end', () => resolve(Buffer.concat(chunks)));
            response.on('error', reject);
        });
        request.on('timeout', () => request.destroy(new Error(`Download timed out after ${timeoutMs}ms: ${url}`)));
        request.on('error', reject);
    });
}
