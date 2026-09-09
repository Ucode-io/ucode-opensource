package v1

import "github.com/Ucode-io/ucode-opensource/services/gateway/api/models"

// The whole builder assistant is injected verbatim — client, hook AND the widget —
// so response parsing, conversation memory and the look are correct and consistent
// on every generated app. The model's only job is to mount <BuilderAssistantWidget/>.
const (
	builderAgentClientFilePath = "src/lib/builderAgentClient.ts"
	useBuilderAgentFilePath    = "src/hooks/useBuilderAgent.ts"
	builderAgentWidgetFilePath = "src/components/BuilderAssistantWidget.tsx"
)

// builderAgentTemplateFiles returns the authoritative template files: when merged
// with the model's output, these win over any same-path file it produced.
func builderAgentTemplateFiles() []models.ProjectFile {
	return []models.ProjectFile{
		{Path: builderAgentClientFilePath, Content: builderAgentClientTemplate},
		{Path: useBuilderAgentFilePath, Content: useBuilderAgentTemplate},
		{Path: builderAgentWidgetFilePath, Content: builderAgentWidgetTemplate},
	}
}

// builderAgentTemplatePaths lists the injected paths so the model knows not to
// re-emit them.
func builderAgentTemplatePaths() []string {
	return []string{builderAgentClientFilePath, useBuilderAgentFilePath, builderAgentWidgetFilePath}
}

const builderAgentClientTemplate = `import apiClient from '@/config/axios';

/** One progress event streamed while the builder assistant works. */
export interface BuilderEvent {
  type: string;
  message?: string;
  value?: string;
  icon?: string;
  percent?: number;
  data?: unknown;
}

/** Tally of everything the assistant built during a turn. */
export interface BuilderSummary {
  tables: number;
  login_tables: number;
  client_types: number;
  roles: number;
  fields: number;
  relations: number;
  menus: number;
  items: number;
}

export interface BuilderChatMessage {
  role: 'user' | 'assistant';
  content: string;
}

export interface SendBuilderResult {
  reply: string;
  summary?: BuilderSummary;
}

export interface SendBuilderHandlers {
  /** Called for every progress event as the assistant streams its work. */
  onEvent?: (event: BuilderEvent) => void;
  /** Abort the in-flight request (e.g. when the user closes the widget). */
  signal?: AbortSignal;
}

/**
 * sendBuilderMessage streams one message to the built-in u-code builder assistant.
 * Build steps arrive through handlers.onEvent while the request is in flight; the
 * promise resolves with the assistant's final reply and a summary of what it built.
 * The conversation is stateless — pass prior turns as history for follow-up context.
 *
 * The request goes through the app's shared axios instance (so it authenticates and
 * uses the same base URL as every other call) and reads the SSE stream incrementally
 * via onDownloadProgress. If a proxy buffers the response, the frames are parsed from
 * the final body instead — the reply is delivered either way.
 */
export async function sendBuilderMessage(
  content: string,
  history: BuilderChatMessage[] = [],
  handlers: SendBuilderHandlers = {},
): Promise<SendBuilderResult> {
  let cursor = 0;
  let reply = '';
  let summary: BuilderSummary | undefined;
  let streamError: string | undefined;

  const handleFrame = (frame: string): void => {
    // One SSE frame; only the "data:" line matters (": keepalive" comments are ignored).
    const dataLine = frame.split('\n').find((line) => line.startsWith('data:'));
    if (!dataLine) return;

    const payload = dataLine.slice(5).trim();
    if (!payload) return;

    let event: BuilderEvent;
    try {
      event = JSON.parse(payload) as BuilderEvent;
    } catch {
      return;
    }

    handlers.onEvent?.(event);

    if (event.type === 'done') {
      reply = event.message ?? reply;
      const data = event.data as { reply?: string; summary?: BuilderSummary } | undefined;
      if (data?.reply) reply = data.reply;
      if (data?.summary) summary = data.summary;
    } else if (event.type === 'error') {
      streamError = event.message || 'Builder failed';
    }
  };

  // Consume every complete '\n\n'-terminated frame past the cursor. Called on each
  // progress tick (accumulating body) and once more on the final body, so it is safe
  // to run repeatedly against the same growing text.
  const consume = (text: string): void => {
    let boundary = text.indexOf('\n\n', cursor);
    while (boundary !== -1) {
      handleFrame(text.slice(cursor, boundary));
      cursor = boundary + 2;
      boundary = text.indexOf('\n\n', cursor);
    }
  };

  const response = await apiClient.post('/v2/ai-builder/messages?stream=true', { content, history }, {
    responseType: 'text',
    headers: { Accept: 'text/event-stream' },
    // A build can take a while server-side; never let the client cancel it early.
    timeout: 25 * 60 * 1000,
    signal: handlers.signal,
    onDownloadProgress: (progressEvent) => {
      const xhr = progressEvent.event?.target as XMLHttpRequest | undefined;
      if (typeof xhr?.responseText === 'string') consume(xhr.responseText);
    },
  });

  if (typeof response?.data === 'string') consume(response.data);

  if (streamError) throw new Error(streamError);
  return { reply, summary };
}

export default sendBuilderMessage;
`

const useBuilderAgentTemplate = `import { useCallback, useRef, useState } from 'react';
import {
  sendBuilderMessage,
  type BuilderChatMessage,
  type BuilderEvent,
  type BuilderSummary,
} from '@/lib/builderAgentClient';

export interface BuilderMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  /** Present on the assistant turn that built something: what it created. */
  summary?: BuilderSummary;
}

export interface UseBuilderAgentResult {
  messages: BuilderMessage[];
  /** Live build steps for the in-flight turn; cleared when the turn completes. */
  steps: BuilderEvent[];
  isLoading: boolean;
  error: string | null;
  send: (text: string) => Promise<void>;
  reset: () => void;
}

// The server keeps a bounded context window, so cap the history we replay.
const HISTORY_LIMIT = 20;

let counter = 0;
const nextId = (): string => 'b' + Date.now().toString() + '_' + (counter++).toString();

function toHistory(messages: BuilderMessage[]): BuilderChatMessage[] {
  return messages
    .slice(-HISTORY_LIMIT)
    .map((m) => ({ role: m.role, content: m.content }));
}

/**
 * useBuilderAgent manages a chat session with the built-in u-code builder assistant:
 * the transcript, the live build steps of the current turn, a send() action, and
 * loading/error state.
 */
export function useBuilderAgent(): UseBuilderAgentResult {
  const [messages, setMessages] = useState<BuilderMessage[]>([]);
  const [steps, setSteps] = useState<BuilderEvent[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const messagesRef = useRef<BuilderMessage[]>([]);
  messagesRef.current = messages;

  const send = useCallback(async (text: string) => {
    const trimmed = text.trim();
    if (!trimmed || isLoading) return;

    setError(null);
    setSteps([]);
    setIsLoading(true);

    const history = toHistory(messagesRef.current);
    setMessages((prev) => [...prev, { id: nextId(), role: 'user', content: trimmed }]);

    try {
      const { reply, summary } = await sendBuilderMessage(trimmed, history, {
        onEvent: (event) => {
          if (event.type === 'done' || event.type === 'error' || event.type === 'provider') return;
          if (!event.message && !event.value) return;
          setSteps((prev) => [...prev, event]);
        },
      });
      const assistant: BuilderMessage = { id: nextId(), role: 'assistant', content: reply };
      if (summary) assistant.summary = summary;
      setMessages((prev) => [...prev, assistant]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong');
    } finally {
      setSteps([]);
      setIsLoading(false);
    }
  }, [isLoading]);

  const reset = useCallback(() => {
    setMessages([]);
    setSteps([]);
    setError(null);
  }, []);

  return { messages, steps, isLoading, error, send, reset };
}

export default useBuilderAgent;
`

const builderAgentWidgetTemplate = `import { useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, KeyboardEvent } from 'react';
import { useBuilderAgent } from '@/hooks/useBuilderAgent';
import type { BuilderMessage } from '@/hooks/useBuilderAgent';
import type { BuilderEvent, BuilderSummary } from '@/lib/builderAgentClient';

export interface BuilderAssistantWidgetProps {
  /** Header title. Defaults to "AI-ассистент". */
  title?: string;
  /** Accent color used for the launcher, header and user bubbles. */
  accentColor?: string;
  /** First-line greeting shown in the empty state. */
  greeting?: string;
}

const ACCENT = '#4f46e5';

// Russian plural picker: forms = [one, few, many].
function ruPlural(n: number, forms: [string, string, string]): string {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return forms[0];
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return forms[1];
  return forms[2];
}

const SUMMARY_FORMS: Array<[keyof BuilderSummary, [string, string, string]]> = [
  ['tables', ['таблица', 'таблицы', 'таблиц']],
  ['login_tables', ['таблица входа', 'таблицы входа', 'таблиц входа']],
  ['fields', ['поле', 'поля', 'полей']],
  ['relations', ['связь', 'связи', 'связей']],
  ['menus', ['раздел', 'раздела', 'разделов']],
  ['items', ['запись', 'записи', 'записей']],
  ['client_types', ['тип клиента', 'типа клиента', 'типов клиента']],
  ['roles', ['роль', 'роли', 'ролей']],
];

function summaryChips(summary?: BuilderSummary): string[] {
  if (!summary) return [];
  const chips: string[] = [];
  for (const [key, forms] of SUMMARY_FORMS) {
    const n = summary[key] || 0;
    if (n > 0) chips.push(n + ' ' + ruPlural(n, forms));
  }
  return chips;
}

// Three backticks — built at runtime so this stays inside the Go raw string.
const FENCE = String.fromCharCode(96, 96, 96);

const mdTableWrap: CSSProperties = { overflowX: 'auto', margin: '6px 0', border: '1px solid #e5e7eb', borderRadius: 8 };
const mdTable: CSSProperties = { borderCollapse: 'collapse', width: '100%', fontSize: 12 };
const mdTh: CSSProperties = { textAlign: 'left', padding: '6px 8px', background: '#f3f4f6', borderBottom: '1px solid #e5e7eb', whiteSpace: 'nowrap', fontWeight: 600 };
const mdTd: CSSProperties = { padding: '6px 8px', borderBottom: '1px solid #f0f0f0', whiteSpace: 'nowrap' };
const mdList: CSSProperties = { margin: '4px 0', paddingLeft: 18 };
const mdHeading: CSSProperties = { fontWeight: 700, margin: '6px 0 2px' };
const mdParagraph: CSSProperties = { margin: '4px 0' };

// inlineBold renders **bold** spans without dangerouslySetInnerHTML.
function inlineBold(text: string) {
  return text.split('**').map((seg, i) => (i % 2 === 1 ? <strong key={i}>{seg}</strong> : <span key={i}>{seg}</span>));
}

function splitRow(line: string): string[] {
  return line.trim().replace(/^\||\|$/g, '').split('|').map((c) => c.trim());
}

function isTableSeparator(line: string): boolean {
  return line.includes('|') && line.includes('-') && /^[\s|:-]+$/.test(line);
}

function isBlockStart(line: string): boolean {
  return line.includes('|') || /^\s*[-*]\s+/.test(line) || /^\s*\d+\.\s+/.test(line) || /^\s*#{1,4}\s+/.test(line);
}

// renderMarkdown turns the assistant's reply into readable UI: paragraphs, **bold**,
// bullet/numbered lists, headings and tables (tables scroll horizontally so they never
// overflow the narrow bubble). Stray code fences are dropped.
function renderMarkdown(text: string) {
  const lines = text.replace(/\r/g, '').split('\n').filter((l) => !l.trim().startsWith(FENCE));
  const out: JSX.Element[] = [];
  let i = 0;
  let key = 0;

  while (i < lines.length) {
    const line = lines[i];

    if (line.trim() === '') { i++; continue; }

    if (line.includes('|') && i + 1 < lines.length && isTableSeparator(lines[i + 1])) {
      const header = splitRow(line);
      i += 2;
      const rows: string[][] = [];
      while (i < lines.length && lines[i].includes('|') && lines[i].trim() !== '') { rows.push(splitRow(lines[i])); i++; }
      out.push(
        <div key={key++} style={mdTableWrap}>
          <table style={mdTable}>
            <thead><tr>{header.map((h, hi) => <th key={hi} style={mdTh}>{inlineBold(h)}</th>)}</tr></thead>
            <tbody>{rows.map((r, ri) => <tr key={ri}>{r.map((c, ci) => <td key={ci} style={mdTd}>{inlineBold(c)}</td>)}</tr>)}</tbody>
          </table>
        </div>,
      );
      continue;
    }

    if (/^\s*[-*]\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^\s*[-*]\s+/.test(lines[i])) { items.push(lines[i].replace(/^\s*[-*]\s+/, '')); i++; }
      out.push(<ul key={key++} style={mdList}>{items.map((it, ii) => <li key={ii}>{inlineBold(it)}</li>)}</ul>);
      continue;
    }

    if (/^\s*\d+\.\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^\s*\d+\.\s+/.test(lines[i])) { items.push(lines[i].replace(/^\s*\d+\.\s+/, '')); i++; }
      out.push(<ol key={key++} style={mdList}>{items.map((it, ii) => <li key={ii}>{inlineBold(it)}</li>)}</ol>);
      continue;
    }

    const heading = line.match(/^\s*#{1,4}\s+(.*)$/);
    if (heading) { out.push(<div key={key++} style={mdHeading}>{inlineBold(heading[1])}</div>); i++; continue; }

    const para: string[] = [line];
    i++;
    while (i < lines.length && lines[i].trim() !== '' && !isBlockStart(lines[i])) { para.push(lines[i]); i++; }
    out.push(
      <p key={key++} style={mdParagraph}>
        {para.map((pl, pi) => <span key={pi}>{pi > 0 ? <br /> : null}{inlineBold(pl)}</span>)}
      </p>,
    );
  }

  return out;
}

function stepText(event: BuilderEvent): string {
  return [event.message, event.value].filter(Boolean).join(': ');
}

export function BuilderAssistantWidget({ title = 'AI-ассистент', accentColor = ACCENT, greeting }: BuilderAssistantWidgetProps) {
  const [open, setOpen] = useState(false);
  const [input, setInput] = useState('');
  const { messages, steps, isLoading, error, send } = useBuilderAgent();
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [messages, steps, isLoading, open]);

  const emptyGreeting = greeting ||
    'Опишите, что нужно построить — я создам таблицы, поля, связи и разделы, добавлю записи и отвечу на вопросы о данных.';

  const submit = () => {
    const text = input.trim();
    if (!text || isLoading) return;
    setInput('');
    void send(text);
  };

  const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
  };

  const styles = useMemo(() => makeStyles(accentColor), [accentColor]);

  return (
    <>
      <style>{WIDGET_CSS}</style>

      {!open && (
        <button type="button" style={styles.launcher} className="bldr-launcher" onClick={() => setOpen(true)} aria-label={title}>
          <span style={styles.launcherDot} />
          {title}
        </button>
      )}

      {open && (
        <div style={styles.panel} className="bldr-panel" role="dialog" aria-label={title}>
          <div style={styles.header}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontWeight: 600 }}>
              <span style={styles.headerDot} />
              {title}
            </div>
            <button type="button" style={styles.iconBtn} className="bldr-icon" onClick={() => setOpen(false)} aria-label="Закрыть">×</button>
          </div>

          <div ref={scrollRef} style={styles.body} className="bldr-scroll">
            {messages.length === 0 && !isLoading && (
              <div style={styles.empty}>{emptyGreeting}</div>
            )}

            {messages.map((m: BuilderMessage) => (
              <div key={m.id} style={m.role === 'user' ? styles.rowUser : styles.rowAssistant}>
                <div style={m.role === 'user' ? styles.bubbleUser : styles.bubbleAssistant}>
                  {renderMarkdown(m.content)}
                  {m.summary && summaryChips(m.summary).length > 0 && (
                    <div style={styles.chips}>
                      {summaryChips(m.summary).map((c, i) => (
                        <span key={i} style={styles.chip}>{c}</span>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            ))}

            {isLoading && (
              <div style={styles.rowAssistant}>
                <div style={styles.steps}>
                  <div style={styles.stepsHead}>
                    <span className="bldr-pulse" style={styles.pulse} />
                    Работаю…
                  </div>
                  {steps.slice(-6).map((s, i) => (
                    <div key={i} style={styles.step}>{stepText(s)}</div>
                  ))}
                </div>
              </div>
            )}

            {error && <div style={styles.error}>{error}</div>}
          </div>

          <div style={styles.footer}>
            <textarea
              rows={1}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={onKeyDown}
              placeholder="Например: создай таблицу клиентов с именем и телефоном"
              style={styles.input}
              disabled={isLoading}
            />
            <button type="button" onClick={submit} disabled={isLoading || !input.trim()} style={styles.send} className="bldr-send">
              →
            </button>
          </div>
        </div>
      )}
    </>
  );
}

const WIDGET_CSS =
  '.bldr-launcher,.bldr-panel,.bldr-panel *{box-sizing:border-box}' +
  '.bldr-launcher{transition:transform .15s ease,box-shadow .15s ease}' +
  '.bldr-launcher:hover{transform:translateY(-2px)}' +
  '.bldr-send:disabled{opacity:.45;cursor:default}' +
  '.bldr-icon:hover{opacity:.75}' +
  '.bldr-scroll::-webkit-scrollbar{width:8px}' +
  '.bldr-scroll::-webkit-scrollbar-thumb{background:#d1d5db;border-radius:8px}' +
  '@keyframes bldrPulse{0%,100%{opacity:1}50%{opacity:.35}}' +
  '.bldr-pulse{animation:bldrPulse 1s ease-in-out infinite}' +
  '@media (max-width:480px){.bldr-panel{inset:0 !important;width:100% !important;height:100% !important;max-width:none !important;max-height:none !important;border-radius:0 !important}}';

function makeStyles(accent: string): Record<string, CSSProperties> {
  return {
    launcher: {
      position: 'fixed', bottom: 24, right: 24, zIndex: 2147483000,
      display: 'flex', alignItems: 'center', gap: 8,
      height: 52, padding: '0 20px', border: 'none', borderRadius: 999,
      background: accent, color: '#fff', fontWeight: 600, fontSize: 15,
      fontFamily: 'inherit', cursor: 'pointer', boxShadow: '0 8px 24px rgba(0,0,0,.18)',
    },
    launcherDot: { width: 8, height: 8, borderRadius: 999, background: '#fff' },
    panel: {
      position: 'fixed', bottom: 24, right: 24, zIndex: 2147483000,
      width: 384, maxWidth: 'calc(100vw - 32px)', height: 600, maxHeight: 'calc(100vh - 48px)',
      display: 'flex', flexDirection: 'column', overflow: 'hidden',
      background: '#fff', color: '#111827', borderRadius: 16,
      boxShadow: '0 16px 48px rgba(0,0,0,.24)', fontFamily: 'inherit',
    },
    header: {
      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      padding: '14px 16px', background: accent, color: '#fff', fontSize: 15,
    },
    headerDot: { width: 8, height: 8, borderRadius: 999, background: '#fff' },
    iconBtn: {
      border: 'none', background: 'transparent', color: '#fff',
      fontSize: 22, lineHeight: 1, cursor: 'pointer', padding: 0,
    },
    body: {
      flex: 1, overflowY: 'auto', padding: 16, background: '#f9fafb',
      display: 'flex', flexDirection: 'column', gap: 10,
    },
    empty: { color: '#6b7280', fontSize: 14, lineHeight: 1.5, padding: '8px 4px' },
    rowUser: { display: 'flex', justifyContent: 'flex-end' },
    rowAssistant: { display: 'flex', justifyContent: 'flex-start' },
    bubbleUser: {
      maxWidth: '85%', padding: '10px 13px', borderRadius: '14px 14px 4px 14px',
      background: accent, color: '#fff', fontSize: 14, lineHeight: 1.45, wordBreak: 'break-word',
    },
    bubbleAssistant: {
      maxWidth: '90%', padding: '10px 13px', borderRadius: '14px 14px 14px 4px',
      background: '#fff', color: '#111827', border: '1px solid #e5e7eb',
      fontSize: 14, lineHeight: 1.45, wordBreak: 'break-word',
    },
    chips: { display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 8 },
    chip: {
      fontSize: 12, padding: '2px 8px', borderRadius: 999,
      background: '#eef2ff', color: '#4338ca', fontWeight: 500,
    },
    steps: {
      maxWidth: '90%', padding: '10px 13px', borderRadius: 12,
      background: '#fff', border: '1px solid #e5e7eb', fontSize: 13, color: '#374151',
    },
    stepsHead: { display: 'flex', alignItems: 'center', gap: 8, fontWeight: 600, marginBottom: 6, color: '#111827' },
    pulse: { width: 8, height: 8, borderRadius: 999, background: accent },
    step: { padding: '2px 0', color: '#4b5563' },
    error: {
      fontSize: 13, color: '#b91c1c', background: '#fef2f2',
      border: '1px solid #fecaca', borderRadius: 10, padding: '8px 10px',
    },
    footer: { display: 'flex', gap: 8, padding: 12, borderTop: '1px solid #eee', background: '#fff' },
    input: {
      flex: 1, resize: 'none', maxHeight: 120, border: '1px solid #d1d5db', borderRadius: 10,
      padding: '10px 12px', fontFamily: 'inherit', fontSize: 14, color: '#111827', outline: 'none',
    },
    send: {
      width: 44, border: 'none', borderRadius: 10, background: accent,
      color: '#fff', fontSize: 18, cursor: 'pointer',
    },
  };
}

export default BuilderAssistantWidget;
`
