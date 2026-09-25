import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import * as vscode from 'vscode';
import { LanguageClient, LanguageClientOptions, ServerOptions } from 'vscode-languageclient/node';
import {
    archiveName, detectPlatform, download, extractFile, lspBinaryName, releasesUrl, resolveReleaseTag, verifyChecksum,
} from './release';

let client: LanguageClient;
let statusBarItem: vscode.StatusBarItem;
let log: vscode.OutputChannel;
const jarScheme = 'krit-jar';

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

    context.subscriptions.push(vscode.workspace.registerTextDocumentContentProvider(
        jarScheme,
        new KritJarContentProvider(() => client),
    ));

    const serverOptions: ServerOptions = {
        command: binaryPath,
        args: ['--verbose'],
    };

    const clientOptions: LanguageClientOptions = {
        documentSelector: [
            { scheme: 'file', language: 'kotlin' },
            { scheme: 'file', pattern: '**/*.kts' },
            { scheme: jarScheme, language: 'kotlin' },
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

class KritJarContentProvider implements vscode.TextDocumentContentProvider {
    constructor(private readonly getClient: () => LanguageClient | undefined) {}

    async provideTextDocumentContent(uri: vscode.Uri): Promise<string> {
        const activeClient = this.getClient();
        if (!activeClient) {
            throw new Error('Krit language client is not running');
        }
        const result = await activeClient.sendRequest<{ uri: string; languageId: string; text: string }>(
            'krit/jarContent',
            { uri: uri.toString(true) },
        );
        return result.text;
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

async function downloadBinary(version: string, binDir: string, binPath: string): Promise<void> {
    const platform = detectPlatform();

    await vscode.window.withProgress(
        {
            location: vscode.ProgressLocation.Notification,
            title: 'Downloading krit-lsp...',
            cancellable: false,
        },
        async () => {
            const tag = await resolveReleaseTag(version);
            const archive = archiveName(tag, platform);
            const baseUrl = `${releasesUrl}/download/${tag}`;

            const [archiveBytes, checksums] = await Promise.all([
                download(`${baseUrl}/${archive}`),
                download(`${baseUrl}/checksums.txt`),
            ]);
            verifyChecksum(archiveBytes, checksums.toString('utf-8'), archive);
            const binary = extractFile(archiveBytes, archive, lspBinaryName(platform));

            // Write to a temp file and rename so an interrupted install never
            // leaves a truncated binary that ensureBinary() would pick up.
            fs.mkdirSync(binDir, { recursive: true });
            const tmpPath = `${binPath}.${process.pid}.tmp`;
            try {
                fs.writeFileSync(tmpPath, binary, { mode: 0o755 });
                fs.renameSync(tmpPath, binPath);
            } finally {
                fs.rmSync(tmpPath, { force: true });
            }
        }
    );

    vscode.window.showInformationMessage('krit-lsp downloaded successfully.');
}
