package v1

import "github.com/Ucode-io/ucode-opensource/services/gateway/api/models"

// Agent integration template files. These are the correctness-critical networking
// layer for talking to a server-side agent from the generated frontend: a thin
// runAgent() client over the project's shared axios instance, and a useAgent()
// chat-session hook. We own and inject them verbatim so the model never has to
// (re)invent the wire protocol — it only builds the UI that consumes them.
const (
	agentClientFilePath = "src/lib/agentClient.ts"
	useAgentFilePath    = "src/hooks/useAgent.ts"
)

// agentTemplateFiles returns the template files to inject into the project. They are
// authoritative: when merged with the model's output, these win over any same-path
// file the model might have produced.
func agentTemplateFiles() []models.ProjectFile {
	return []models.ProjectFile{
		{Path: agentClientFilePath, Content: agentClientTemplate},
		{Path: useAgentFilePath, Content: useAgentTemplate},
	}
}

// agentTemplatePaths lists the injected file paths, for telling the model which
// files already exist and must not be re-emitted.
func agentTemplatePaths() []string {
	return []string{agentClientFilePath, useAgentFilePath}
}

const agentClientTemplate = `import apiClient from '@/config/axios';

export interface AgentRunStep {
  index: number;
  tool_name: string;
  tool_input: Record<string, unknown>;
  tool_result: string;
  is_error: boolean;
}

export interface AgentRun {
  id: string;
  agent_id: string;
  status: 'running' | 'succeeded' | 'failed';
  output: string;
  steps: AgentRunStep[];
  tokens_used: number;
  error: string;
}

/** A document the agent generated during the run (e.g. a PDF) for the user to download. */
export interface AgentFile {
  name: string;
  url: string;
  content_type: string;
}

export interface RunAgentResult {
  reply: string;
  run: AgentRun;
  files: AgentFile[];
}

/**
 * runAgent sends a single message to a server-side AI agent and resolves with its
 * reply plus any files it generated. Pass optional structured context (e.g. the
 * record the user is viewing) so the agent can ground its answer.
 */
export async function runAgent(
  agentId: string,
  message: string,
  context?: Record<string, unknown>,
): Promise<RunAgentResult> {
  // Agent runs can take up to 20 minutes server-side — override the shared
  // axios instance timeout so the frontend never cancels a long-running run.
  const res = await apiClient.post('/v2/agents/' + agentId + '/run', { message, context }, {
    timeout: 25 * 60 * 1000,
  });
  const payload = (res?.data?.data ?? {}) as AgentRun & { files?: AgentFile[] };
  if (payload.status === 'failed') {
    throw new Error(payload.error || 'Agent run failed');
  }
  const files = Array.isArray(payload.files) ? payload.files : [];
  return { reply: payload.output ?? '', run: payload as AgentRun, files };
}

/**
 * openAgentFile opens a file the agent generated in a new browser tab.
 * The browser's native PDF viewer provides its own Save/Download button —
 * no cross-origin download tricks needed. The URL is also persisted in the
 * database record so the user can always retrieve it later.
 *
 * @deprecated alias downloadAgentFile kept for backwards compatibility
 */
export function openAgentFile(file: AgentFile): void {
  if (!file?.url) return;
  window.open(file.url, '_blank', 'noopener,noreferrer');
}

/** @alias openAgentFile */
export const downloadAgentFile = openAgentFile;

export default runAgent;
`

const useAgentTemplate = `import { useCallback, useRef, useState } from 'react';
import { runAgent, downloadAgentFile, type AgentFile } from '@/lib/agentClient';

export interface AgentMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  /** Files the agent generated for this reply (e.g. a PDF). Render a download link for each. */
  files?: AgentFile[];
}

export interface UseAgentOptions {
  /** Optional structured context sent with every message (e.g. the current record). */
  context?: Record<string, unknown>;
  /** When true, generated files are also downloaded automatically as they arrive. */
  autoDownloadFiles?: boolean;
}

export interface UseAgentResult {
  messages: AgentMessage[];
  isLoading: boolean;
  error: string | null;
  send: (text: string) => Promise<void>;
  reset: () => void;
}

let counter = 0;
const nextId = (): string => 'm' + Date.now().toString() + '_' + (counter++).toString();

/**
 * useAgent manages a chat session with a server-side AI agent identified by agentId.
 * It keeps the running transcript, exposes a send() action, and surfaces loading and
 * error state for the UI.
 */
export function useAgent(agentId: string, options: UseAgentOptions = {}): UseAgentResult {
  const [messages, setMessages] = useState<AgentMessage[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const contextRef = useRef(options.context);
  contextRef.current = options.context;
  const autoDownloadRef = useRef(options.autoDownloadFiles);
  autoDownloadRef.current = options.autoDownloadFiles;

  const send = useCallback(
    async (text: string) => {
      const trimmed = text.trim();
      if (!trimmed || isLoading) return;

      setError(null);
      setIsLoading(true);
      setMessages((prev) => [...prev, { id: nextId(), role: 'user', content: trimmed }]);

      try {
        const { reply, files } = await runAgent(agentId, trimmed, contextRef.current);
        const assistant: AgentMessage = { id: nextId(), role: 'assistant', content: reply };
        if (files.length > 0) {
          assistant.files = files;
          if (autoDownloadRef.current) files.forEach(downloadAgentFile);
        }
        setMessages((prev) => [...prev, assistant]);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Something went wrong');
      } finally {
        setIsLoading(false);
      }
    },
    [agentId, isLoading],
  );

  const reset = useCallback(() => {
    setMessages([]);
    setError(null);
  }, []);

  return { messages, isLoading, error, send, reset };
}

export default useAgent;
`
