import { useCallback, useEffect, useReducer, useRef } from "react";
import {
  createChat,
  fetchMessages,
  streamMessage,
  useForgetSessions,
  useRefreshProject,
} from "../api/chat";
import {
  applyEvent,
  emptyRun,
  nextId,
  summaryCount,
  type Message,
  type Run,
  type Step,
  type StreamEvent,
} from "./run";

/**
 * Состояние беседы с помощником.
 *
 * Не в TanStack Query: ответ приходит потоком и меняется десятки раз
 * за реплику, а кэш запросов — про готовые ответы. В кэше живёт только
 * список прошлых бесед (`api/chat.ts`).
 *
 * Живёт в панели и переживает её закрытие: панель остаётся смонтированной
 * и просто ничего не рисует. Закрыть окно и вернуться к недописанному
 * разговору — обычное дело.
 */
type State = {
  chatId: string;
  /** Заголовок беседы. Пусто — беседа ещё не начата. */
  title: string;
  messages: Message[];
  /** Идущая прямо сейчас реплика помощника. null — тишина. */
  run: Run | null;
};

type Action =
  | { type: "open"; chatId: string; title: string }
  | { type: "loaded"; messages: Message[] }
  | { type: "ask"; content: string }
  | { type: "chat"; chatId: string }
  | { type: "event"; event: StreamEvent }
  | { type: "finish"; error?: string; stopped?: boolean }
  | { type: "reset" };

const initial: State = { chatId: "", title: "", messages: [], run: null };

export function reducer(state: State, action: Action): State {
  switch (action.type) {
    case "open":
      return { ...initial, chatId: action.chatId, title: action.title };

    case "loaded":
      return { ...state, messages: action.messages, run: null };

    case "chat":
      return { ...state, chatId: action.chatId };

    case "ask":
      return {
        ...state,
        // Первая реплика и есть имя беседы — так её называет и сервер
        // (`ai_ucode_chat.go:maybeTitleChat`).
        title: state.title || action.content,
        messages: [
          ...state.messages,
          { id: nextId(), role: "user", content: action.content },
        ],
        run: emptyRun(),
      };

    case "event":
      return state.run ? { ...state, run: applyEvent(state.run, action.event) } : state;

    /*
     * Законченная реплика переезжает из `run` в список сообщений вместе
     * со своими шагами: свернуть их в текст нельзя — человек читает
     * список созданного как отчёт о работе.
     */
    case "finish": {
      if (!state.run) return state;

      const stopped = action.stopped ?? false;
      const run: Run = {
        ...state.run,
        active: false,
        stopped,
        error: state.run.error || action.error || null,
        /*
         * Незакрытый шаг после остановки — уже не «идёт»: крутящийся
         * кружок в нём остался бы навсегда и врал бы. Статус снимается,
         * а сам шаг остаётся: он был.
         */
        steps: stopped ? state.run.steps.map(freeze) : state.run.steps,
      };

      return {
        ...state,
        run: null,
        messages: [
          ...state.messages,
          { id: nextId(), role: "assistant", content: run.content, run },
        ],
      };
    }

    case "reset":
      return initial;
  }
}

/** Снимает «идёт» с шага: поток больше не читают, и продолжения не будет. */
function freeze(step: Step): Step {
  if (step.status !== "started") return step;

  const frozen = { ...step };
  delete frozen.status;
  return frozen;
}

export function useCopilotChat() {
  const [state, dispatch] = useReducer(reducer, initial);
  const abort = useRef<AbortController | null>(null);
  const forgetSessions = useForgetSessions();
  const refreshProject = useRefreshProject();

  // Уход со страницы обрывает чтение потока, но не саму работу: сборка
  // идёт на сервере и доживёт до следующего открытия беседы.
  useEffect(() => () => abort.current?.abort(), []);

  const sending = state.run !== null;

  const send = useCallback(
    async (text: string) => {
      const content = text.trim();
      if (!content || sending) return;

      dispatch({ type: "ask", content });

      const controller = new AbortController();
      abort.current = controller;

      try {
        let chatId = state.chatId;
        if (!chatId) {
          chatId = await createChat(content);
          if (!chatId) throw new Error("chat id is empty");

          dispatch({ type: "chat", chatId });
          // В истории появилась новая беседа — список устарел.
          forgetSessions();
        }

        // Помощник правит схему проекта, а не отвечает словами: если он
        // что-то создал, показанное на экране устарело.
        let changed = false;

        await streamMessage({
          chatId,
          content,
          signal: controller.signal,
          onEvent: (event) => {
            if (event.type === "done" && summaryCount(event.data?.summary ?? null) > 0) {
              changed = true;
            }
            dispatch({ type: "event", event });
          },
        });

        dispatch({ type: "finish" });
        if (changed) refreshProject();
      } catch (error) {
        if (controller.signal.aborted) return;

        dispatch({ type: "finish", error: (error as Error).message });
      } finally {
        abort.current = null;
      }
    },
    [forgetSessions, refreshProject, sending, state.chatId],
  );

  /**
   * Перестать ждать ответ.
   *
   * Останавливается ЧТЕНИЕ потока, а не работа: сборка идёт на сервере
   * и доводится до конца — там же она и сохранится. Поэтому кнопка
   * говорит «остановить», а под остановленной репликой стоит оговорка,
   * а не «отменено».
   */
  const stop = useCallback(() => {
    abort.current?.abort();
    abort.current = null;
    dispatch({ type: "finish", stopped: true });
  }, []);

  /** Открыть прошлую беседу. Шаги в ней не сохранены — только текст. */
  const open = useCallback(async (chatId: string, title: string) => {
    abort.current?.abort();
    dispatch({ type: "open", chatId, title });

    dispatch({ type: "loaded", messages: await fetchMessages(chatId).catch(() => []) });
  }, []);

  const reset = useCallback(() => {
    abort.current?.abort();
    dispatch({ type: "reset" });
  }, []);

  return { ...state, sending, send, stop, open, reset };
}
