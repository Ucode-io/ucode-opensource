import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ensureAccessToken } from "@/shared/api/client";
import { session } from "@/shared/api/session";
import { useSession } from "@/shared/api/use-session";
import { keys } from "@/shared/lib/query-keys";
import type { StreamEvent } from "../model/run";

/**
 * Помощник u-code: беседа, в которой ИИ сам заводит таблицы, поля,
 * связи и пункты меню.
 *
 * Ручки — `/v1/ai-chat/*` (`api/api.go:463`). Тип беседы `ucode`
 * обязателен: под тем же адресом живёт чат ugen, который вместо работы
 * над существующим проектом создаёт себе черновой
 * (`ai_chat.go:276` — без `type=ucode` бэкенд заводит MCP-проект).
 *
 * `project-id` уезжает параметром адреса в КАЖДОМ запросе: мидлварь
 * шлюза читает его только оттуда (`middleware_admin.go:62`), а без него
 * `resolveAiChatService` отвечает отказом (`ai_chat.go:122`).
 */
const CHATS = "/v1/ai-chat";

/** Тип беседы. Второй — `ugen`, он не наш. */
const UCODE = "ucode";

export type ChatSession = {
  id: string;
  title: string;
  /** Когда в беседу писали в последний раз. Пусто — сервер не сказал. */
  updatedAt: string;
};

export type HistoryMessage = {
  id: string;
  role: "user" | "assistant";
  content: string;
};

type ChatDto = { id?: string; title?: string; updated_at?: string };
type ChatsDto = { chats?: ChatDto[] | null } | ChatDto[];
type MessageDto = { id?: string; role?: string; content?: string };
type MessagesDto = { messages?: MessageDto[] | null } | MessageDto[];

/** Сколько прошлых бесед показывать. Дальше — не история, а архив. */
const HISTORY_LIMIT = 20;

/**
 * Прошлые беседы проекта. Запрашиваются, только когда историю открыли:
 * список нужен раз в сессию, а панель открыта весь день.
 */
export function useChatSessions(enabled: boolean) {
  const projectId = useSession().getProjectId() ?? "";

  const query = useQuery({
    queryKey: keys.copilot.chats(projectId),
    queryFn: () =>
      api.get<ChatsDto>(`${CHATS}/list`, {
        params: {
          "project-id": projectId,
          type: UCODE,
          order_by: "updated_at",
          order_direction: "desc",
          limit: HISTORY_LIMIT,
          offset: 0,
        },
      }),
    enabled: enabled && Boolean(projectId),
    select: toSessions,
  });

  return { sessions: query.data ?? NO_SESSIONS, isLoading: query.isLoading };
}

const NO_SESSIONS: ChatSession[] = [];

export function toSessions(dto: ChatsDto): ChatSession[] {
  const list = Array.isArray(dto) ? dto : (dto.chats ?? []);

  return list.map((chat) => ({
    id: chat.id ?? "",
    title: chat.title ?? "",
    updatedAt: chat.updated_at ?? "",
  }));
}

/**
 * Помощник поменял схему проекта — всё, что показано, устарело.
 *
 * Без этого «таблица создана» остаётся словами: меню, список таблиц
 * и строки грузятся из кэша, и новой таблицы не видно до перезагрузки
 * страницы. Ровно эту дыру и оставляла старая админка.
 */
export function useRefreshProject() {
  const client = useQueryClient();

  return () => {
    void client.invalidateQueries({ queryKey: keys.menus.all });
    void client.invalidateQueries({ queryKey: keys.tables.all });
    void client.invalidateQueries({ queryKey: keys.items.all });
  };
}

/** Сбросить список бесед: после первой реплики в новой он устарел. */
export function useForgetSessions() {
  const client = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return () => void client.invalidateQueries({ queryKey: keys.copilot.chats(projectId) });
}

/**
 * Беседа заводится лениво — первой репликой, а не открытием панели:
 * иначе каждое случайное нажатие оставляет в истории пустую строку.
 */
export async function createChat(title: string): Promise<string> {
  const chat = await api.post<ChatDto>(
    CHATS,
    { title: title.slice(0, 60), type: UCODE },
    { params: { "project-id": session.getProjectId() ?? "" } },
  );

  return chat?.id ?? "";
}

/** Переписка выбранной беседы. Шаги сборки в ней не хранятся — только текст. */
export async function fetchMessages(chatId: string): Promise<HistoryMessage[]> {
  const dto = await api.get<MessagesDto>(`${CHATS}/messages/${chatId}`, {
    params: { "project-id": session.getProjectId() ?? "" },
  });

  return toMessages(dto, chatId);
}

export function toMessages(dto: MessagesDto, chatId: string): HistoryMessage[] {
  const list = Array.isArray(dto) ? dto : (dto.messages ?? []);

  return list.map((message, index) => ({
    id: message.id || `${chatId}-${index}`,
    role: message.role === "assistant" ? "assistant" : "user",
    content: message.content ?? "",
  }));
}

/**
 * Отправляет реплику и читает поток событий сборки.
 *
 * Не axios и не EventSource: ответ приходит потоком server-sent events,
 * которого axios не отдаёт по кускам, а EventSource не умеет POST с телом.
 * Остаётся fetch с ридером — и заголовки к нему приходится собирать
 * руками, теми же, что ставит перехватчик клиента.
 *
 * Токен доступа живёт только в памяти, поэтому перед запросом он
 * восстанавливается: сюда попадают и после перезагрузки страницы.
 */
export async function streamMessage({
  chatId,
  content,
  onEvent,
  signal,
}: {
  chatId: string;
  content: string;
  onEvent: (event: StreamEvent) => void;
  signal: AbortSignal;
}): Promise<void> {
  await ensureAccessToken();

  const url = new URL(`${import.meta.env.VITE_API_URL}${CHATS}/ucode-messages/${chatId}`);
  url.searchParams.set("stream", "true");
  url.searchParams.set("project-id", session.getProjectId() ?? "");

  const response = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${session.getAccess() ?? ""}`,
      "Environment-Id": session.getEnvironmentId() ?? "",
    },
    body: JSON.stringify({ content }),
    signal,
  });

  if (!response.ok || !response.body) {
    throw new Error((await response.text().catch(() => "")) || `HTTP ${response.status}`);
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });

      // Событие кончается пустой строкой — это и есть весь протокол SSE.
      let split = buffer.indexOf("\n\n");
      for (; split !== -1; split = buffer.indexOf("\n\n")) {
        const frame = buffer.slice(0, split);
        buffer = buffer.slice(split + 2);

        for (const line of frame.split("\n")) {
          // Строка-двоеточие — тиканье, чтобы прокси не закрыл соединение.
          if (!line.startsWith("data:")) continue;

          const raw = line.slice(5).trim();
          if (!raw) continue;

          try {
            onEvent(JSON.parse(raw) as StreamEvent);
          } catch {
            // Порванный кадр — не повод бросать поток: читаем дальше.
          }
        }
      }
    }
  } finally {
    reader.releaseLock();
  }
}
