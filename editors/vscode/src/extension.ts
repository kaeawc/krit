import * as crypto from 'crypto';
import * as fs from 'fs';
import * as https from 'https';
import * as http from 'http';
import * as os from 'os';
import * as path from 'path';
import * as vscode from 'vscode';
import { LanguageClient, LanguageClientOptions, ServerOptions } from 'vscode-languageclient/node';

let client: LanguageClient;
let statusBarItem: vscode.StatusBarItem;
let log: vscode.OutputChannel;

export async function activate(context: vscode.ExtensionContext) {
    log = vscode.window.createOutputChannel('Krit (extension)');
    context.subscriptions.push(log);
    const t0 = Date.now();
    const logLine = (msg: string) => log.appendLine(`[${Date.now() - t0}ms] ${msg}`);
    logLine('activate() entered');

    const config = vscode.workspace.getConfiguration('krit');
    if (!config.get<boolean>('enable', true)) {
        logLine('krit.enable=false — bailing out');
        return;
    }

    statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left);
    statusBarItem.text = '$(loading~spin) Krit';
    statusBarItem.tooltip = 'Krit Kotlin Lint: starting...';
    statusBarItem.show();
    context.subscriptions.push(statusBarItem);
    logLine('status bar shown');

    let binaryPath: string;
    try {
        logLine('ensureBinary: begin');
        binaryPath = await ensureBinary(context);
        logLine(`ensureBinary: resolved ${binaryPath}`);
    } catch (e) {
        logLine(`ensureBinary: FAILED ${e}`);
        statusBarItem.text = '$(error) Krit';
        statusBarItem.tooltip = `Krit Kotlin Lint: binary not available (${e})`;
        return;
    }

    const configPath = config.get<string>('configPath') || '';
    logLine(`configPath=${configPath || '(empty)'}`);

    const serverOptions: ServerOptions = {
        command: binaryPath,
        args: ['--verbose'],
    };

    const clientOptions: LanguageClientOptions = {
        documentSelector: [
            { scheme: 'file', language: 'kotlin' },
            { scheme: 'file', pattern: '**/*.kts' },
        ],
        synchronize: {
            fileEvents: vscode.workspace.createFileSystemWatcher('**/krit.yml'),
        },
        initializationOptions: {
            configPath: configPath,
        },
    };

    client = new LanguageClient('krit', 'Krit Kotlin Lint', serverOptions, clientOptions);
    logLine('LanguageClient constructed; calling start()');

    try {
        await client.start();
        logLine('client.start(): resolved');
        statusBarItem.text = '$(check) Krit';
        statusBarItem.tooltip = 'Krit Kotlin Lint active';
    } catch (e) {
        logLine(`client.start(): FAILED ${e}`);
        statusBarItem.text = '$(error) Krit';
        statusBarItem.tooltip = `Krit Kotlin Lint: failed to start (${e})`;
    }
}

export function deactivate(): Thenable<void> | undefined {
    if (statusBarItem) {
        statusBarItem.dispose();
    }
    return client?.stop();
}

async function ensureBinary(context: vscode.ExtensionContext): Promise<string> {
    const config = vscode.workspace.getConfiguration('krit');
    const customPath = config.get<string>('binaryPath');
    if (customPath && fs.existsSync(customPath)) return customPath;

    // Check common locations
    const candidates = findBinaryCandidates();
    for (const c of candidates) {
        if (fs.existsSync(c)) return c;
    }

    // Check extension storage
    const version = config.get<string>('version', 'latest');
    const binDir = path.join(context.globalStoragePath, 'bin');
    const binName = process.platform === 'win32' ? 'krit-lsp.exe' : 'krit-lsp';
    const binPath = path.join(binDir, binName);

    if (fs.existsSync(binPath)) return binPath;

    // Offer to download
    const choice = await vscode.window.showInformationMessage(
        'krit-lsp binary not found. Download it?',
        'Download', 'Configure Path'
    );

    if (choice === 'Download') {
        await downloadBinary(version, binDir, binPath);
        return binPath;
    } else if (choice === 'Configure Path') {
        await vscode.commands.executeCommand('workbench.action.openSettings', 'krit.binaryPath');
    }

    throw new Error('krit-lsp not available');
}

function findBinaryCandidates(): string[] {
    const home = os.homedir();
    const gopath = process.env.GOPATH || path.join(home, 'go');
    const binName = process.platform === 'win32' ? 'krit-lsp.exe' : 'krit-lsp';

    return [
        path.join(home, '.krit', 'bin', binName),
        path.join(gopath, 'bin', binName),
        path.join('/usr', 'local', 'bin', binName),
    ];
}

function getPlatformSuffix(): string {
    const platform = process.platform === 'darwin' ? 'darwin' :
                     process.platform === 'win32' ? 'windows' : 'linux';
    const arch = process.arch === 'arm64' ? 'arm64' : 'amd64';
    return `${platform}-${arch}`;
}

async function downloadBinary(version: string, binDir: string, binPath: string): Promise<void> {
    const suffix = getPlatformSuffix();
    const ext = process.platform === 'win32' ? '.exe' : '';
    const binaryName = `krit-lsp-${suffix}${ext}`;
    const url = `https://github.com/kaeawc/krit/releases/download/${version}/${binaryName}`;
    const checksumsUrl = `https://github.com/kaeawc/krit/releases/download/${version}/checksums.txt`;

    fs.mkdirSync(binDir, { recursive: true });

    await vscode.window.withProgress(
        {
            location: vscode.ProgressLocation.Notification,
            title: 'Downloading krit-lsp...',
            cancellable: false,
        },
        async () => {
            await downloadFile(url, binPath);
            if (process.platform !== 'win32') {
                fs.chmodSync(binPath, 0o755);
            }

            // Verify checksum (skip gracefully for dev builds)
            const checksumOk = await verifyChecksum(binPath, checksumsUrl, binaryName);
            if (checksumOk === false) {
                fs.unlinkSync(binPath);
                throw new Error('Checksum verification failed for krit-lsp binary');
            }
        }
    );

    vscode.window.showInformationMessage('krit-lsp downloaded successfully.');
}

/**
 * Verify the SHA-256 checksum of a downloaded file against checksums.txt.
 * Returns true if verified, false if mismatch, undefined if checksums unavailable.
 */
async function verifyChecksum(
    binaryPath: string,
    checksumsUrl: string,
    archiveName: string,
): Promise<boolean | undefined> {
    let checksumsText: string;
    try {
        checksumsText = await downloadText(checksumsUrl);
    } catch {
        // checksums.txt not available (dev build) -- skip verification
        return undefined;
    }

    const expectedLine = checksumsText.split('\n').find(l => l.includes(archiveName));
    if (!expectedLine) {
        return undefined;
    }
    const expected = expectedLine.split(/\s+/)[0];

    const hash = crypto.createHash('sha256');
    const data = fs.readFileSync(binaryPath);
    hash.update(data);
    const actual = hash.digest('hex');
    return expected === actual;
}

function downloadText(url: string): Promise<string> {
    return new Promise((resolve, reject) => {
        const request = https.get(url, (response) => {
            if (response.statusCode === 301 || response.statusCode === 302) {
                const redirectUrl = response.headers.location;
                if (!redirectUrl) {
                    reject(new Error('Redirect with no location header'));
                    return;
                }
                downloadText(redirectUrl).then(resolve, reject);
                return;
            }
            if (response.statusCode !== 200) {
                reject(new Error(`Download failed: HTTP ${response.statusCode}`));
                return;
            }
            const chunks: Buffer[] = [];
            response.on('data', (chunk: Buffer) => chunks.push(chunk));
            response.on('end', () => resolve(Buffer.concat(chunks).toString('utf-8')));
        });
        request.on('error', reject);
    });
}

function downloadFile(url: string, dest: string): Promise<void> {
    return new Promise((resolve, reject) => {
        const file = fs.createWriteStream(dest);
        const request = https.get(url, (response) => {
            // Handle redirects
            if (response.statusCode === 301 || response.statusCode === 302) {
                const redirectUrl = response.headers.location;
                if (!redirectUrl) {
                    reject(new Error('Redirect with no location header'));
                    return;
                }
                file.close();
                fs.unlinkSync(dest);
                downloadFile(redirectUrl, dest).then(resolve, reject);
                return;
            }

            if (response.statusCode !== 200) {
                reject(new Error(`Download failed: HTTP ${response.statusCode}`));
                return;
            }

            response.pipe(file);
            file.on('finish', () => {
                file.close();
                resolve();
            });
        });
        request.on('error', (err) => {
            fs.unlink(dest, () => {});
            reject(err);
        });
    });
}
