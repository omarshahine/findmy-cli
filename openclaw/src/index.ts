/**
 * OpenClaw plugin entry for findmy-cli.
 *
 * Registers two tools that shell out to the `findmy` binary to query Find My
 * friend locations on macOS. The CLI drives FindMy.app via screen capture
 * and Vision OCR — see the host repo for the underlying mechanism.
 */

import { definePluginEntry } from 'openclaw/plugin-sdk/plugin-entry';
import { Type } from '@sinclair/typebox';
import { execFileSync, execFile } from 'child_process';
import { promisify } from 'util';
import { existsSync } from 'fs';

const execFileAsync = promisify(execFile);

interface PluginConfig {
	cliPath?: string;
}

interface ToolDef {
	name: string;
	description: string;
	parameters: ReturnType<typeof Type.Object>;
	buildArgs: (params: Record<string, unknown>) => string[];
}

const TOOLS: ToolDef[] = [
	{
		name: 'findmy_people',
		description:
			'List every friend in the FindMy.app People sidebar. Returns name, coarse location (city, state), staleness, and distance for each person. Use for "who is where", "anyone near downtown", or any broad location query. Each entry includes a `staleness` field — if "Paused", the friend has paused location sharing and the location is the last known position.',
		parameters: Type.Object({}),
		buildArgs: () => ['people', '--json'],
	},
	{
		name: 'findmy_person',
		description:
			'Look up a single friend by name (case-insensitive substring match). Returns the same shape as findmy_people but filtered. Use for "where is X", "is X home", "how far is X". Match works on partial names — "sarah" matches "Sarah Shahine".',
		parameters: Type.Object({
			name: Type.String({
				description: 'Friend name or substring (case-insensitive).',
			}),
		}),
		buildArgs: (params) => ['person', String(params.name), '--json'],
	},
];

function toolResult(text: string) {
	return {
		content: [{ type: 'text' as const, text }],
		details: undefined,
	};
}

function whichBinary(name: string): string | null {
	const cmd = process.platform === 'win32' ? 'where.exe' : 'which';
	try {
		const result = execFileSync(cmd, [name], { encoding: 'utf8' }).trim();
		const first = result.split('\n')[0]?.trim();
		return first || null;
	} catch {
		return null;
	}
}

/**
 * Resolve the findmy binary:
 * 1. Plugin config cliPath
 * 2. Env var FINDMY_CLI_PATH
 * 3. PATH lookup
 */
function resolveCliPath(config?: PluginConfig): string {
	if (config?.cliPath && existsSync(config.cliPath)) {
		return config.cliPath;
	}

	const envPath = process.env.FINDMY_CLI_PATH;
	if (envPath && existsSync(envPath)) {
		return envPath;
	}

	const found = whichBinary('findmy');
	if (found) return found;

	throw new Error(
		'findmy not found on PATH. Install with: brew install omarshahine/tap/findmy-cli\n' +
			'Or set FINDMY_CLI_PATH or configure cliPath in plugin settings.'
	);
}

export default definePluginEntry({
	id: 'findmy-cli',
	name: 'Find My',
	description: 'Query Find My friend locations on macOS via UI scraping',

	register(api) {
		const config = api.pluginConfig as PluginConfig | undefined;

		let cliPath: string;
		try {
			cliPath = resolveCliPath(config);
		} catch (error) {
			const errorMessage = error instanceof Error ? error.message : String(error);
			for (const tool of TOOLS) {
				api.registerTool({
					name: tool.name,
					label: tool.name,
					description: tool.description,
					parameters: tool.parameters,
					async execute() {
						return toolResult(
							JSON.stringify({ success: false, error: errorMessage }, null, 2)
						);
					},
				});
			}
			return;
		}

		for (const tool of TOOLS) {
			api.registerTool({
				name: tool.name,
				label: tool.name,
				description: tool.description,
				parameters: tool.parameters,

				async execute(_id: string, params: Record<string, unknown>) {
					try {
						const args = tool.buildArgs(params);
						const { stdout } = await execFileAsync(cliPath, args, {
							encoding: 'utf8',
							// FindMy.app capture is slow on cold boot — it has to launch,
							// switch tabs, render the sidebar, and OCR a screenshot.
							timeout: 60_000,
							maxBuffer: 4 * 1024 * 1024,
						});

						if (stdout.trim().length === 0) {
							return toolResult(JSON.stringify({ success: true }, null, 2));
						}

						let result: unknown;
						try {
							result = JSON.parse(stdout);
						} catch {
							result = { output: stdout.trim() };
						}
						return toolResult(JSON.stringify(result, null, 2));
					} catch (error: unknown) {
						const message = error instanceof Error ? error.message : String(error);
						const stderr =
							error && typeof error === 'object' && 'stderr' in error
								? String((error as { stderr: unknown }).stderr).trim()
								: '';
						const errorOutput = stderr ? `${message}\n\nstderr: ${stderr}` : message;
						return toolResult(
							JSON.stringify({ success: false, error: errorOutput }, null, 2)
						);
					}
				},
			});
		}
	},
});
