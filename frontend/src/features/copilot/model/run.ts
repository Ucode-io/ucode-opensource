/**
 * Одна реплика помощника, собранная из потока событий.
 *
 * Бэкенд не присылает готовый ответ — он рассказывает о работе по ходу:
 * «читаю схему», «создаю таблицу product», «поле price готово», и только
 * в конце шлёт текст. Здесь эти события складываются в то, что видно
 * на экране: список шагов, итог и текст ответа.
 *
 * Свёртка отдельно от React: это чистая функция над событием, и проверить
 * её можно без панели (`run.test.ts`). Формы событий — из
 * `generation_stream.go:96` и `ai_ucode_chat.go:58`.
 */

/** Событие потока. Всё, кроме `type`, необязательно — так его шлёт Go. */
export type StreamEvent = {
  type?: string;
  message?: string;
  value?: string;
  icon?: string;
  data?: {
    action?: string;
    status?: string;
    table?: string;
    reason?: string;
    provider?: string;
    coder_model?: string;
    duration_sec?: number;
    summary?: Summary;
    message?: { content?: string };
  } | null;
};

/** Сколько чего создано за реплику. Все поля — счётчики. */
export type Summary = {
  tables?: number;
  fields?: number;
  relations?: number;
  menus?: number;
  items?: number;
};

/**
 * Шаг работы. `reasoning` — мысль вслух («Понял, начинаю со схемы»),
 * `step` — действие над объектом проекта. Рисуются они по-разному:
 * мысль не имеет состояния, у действия оно есть всегда.
 */
export type Step = {
  id: string;
  kind: "reasoning" | "step";
  icon: string;
  message: string;
  value?: string;
  action?: string;
  status?: string;
  /** Слаг таблицы: по нему «готово» находит свою строку «начал». */
  table?: string;
};

export type Run = {
  /** Поток ещё идёт. */
  active: boolean;
  /**
   * Человек перестал ждать. Не «отменено»: сборка идёт на сервере
   * и доводится до конца — мы всего лишь закрыли поток.
   */
  stopped: boolean;
  /** Кем сгенерировано: провайдер и модель. */
  provider: string;
  steps: Step[];
  summary: Summary | null;
  /** Сколько секунд заняла работа. */
  duration: number | null;
  /** Текст ответа. Приходит последним событием. */
  content: string;
  error: string | null;
};

export type Message = {
  id: string;
  role: "user" | "assistant";
  content: string;
  /** Только у ответа помощника: чем он занимался, пока отвечал. */
  run?: Run;
};

export const emptyRun = (): Run => ({
  active: true,
  stopped: false,
  provider: "",
  steps: [],
  summary: null,
  duration: null,
  content: "",
  error: null,
});

/** Счётчик, а не Date.now(): ключи должны быть стабильны в тестах. */
let counter = 0;
export const nextId = () => `c${++counter}`;

/**
 * Складывает одно событие в реплику. Возвращает НОВЫЙ объект — состояние
 * идёт в React, и правка на месте не вызовет перерисовку.
 */
export function applyEvent(run: Run, event: StreamEvent): Run {
  const data = event.data ?? {};

  switch (event.type) {
    case "provider":
      return {
        ...run,
        provider: [data.provider, data.coder_model].filter(Boolean).join(" · "),
      };

    /*
     * Таблица объявляется дважды: «начал» и «готово». Второе событие
     * ДОПОЛНЯЕТ первое, а не добавляет строку — иначе один и тот же
     * «product» стоит в списке дважды, и непонятно, сделано или нет.
     */
    case "table_done": {
      const at = run.steps.findLastIndex(
        (step) =>
          step.status === "started" &&
          step.action === data.action &&
          step.table === (data.table ?? ""),
      );
      if (at === -1) return { ...run, steps: [...run.steps, toStep(event)] };

      const started = run.steps[at]!;
      const value = event.value || started.value;
      const steps = [...run.steps];
      steps[at] = {
        ...started,
        status: data.status || "done",
        message: event.message || started.message,
        ...(value ? { value } : {}),
      };
      return { ...run, steps };
    }

    // Предупреждение — не конец работы: один инструмент не справился,
    // помощник пробует иначе. Показываем как неудавшийся шаг.
    case "warning":
      return { ...run, steps: [...run.steps, { ...toStep(event), status: "failed" }] };

    case "error":
      return { ...run, active: false, error: event.message || "" };

    case "done":
      return {
        ...run,
        active: false,
        content: data.message?.content || event.message || "",
        summary: data.summary ?? null,
        duration: data.duration_sec ?? null,
      };

    default:
      // Событие без `data` — это мысль вслух, а не действие: у неё нет
      // ни объекта, ни состояния.
      return {
        ...run,
        steps: [
          ...run.steps,
          event.data
            ? toStep(event)
            : {
                id: nextId(),
                kind: "reasoning" as const,
                icon: event.icon || "brain",
                message: event.message ?? "",
              },
        ],
      };
  }
}

function toStep(event: StreamEvent): Step {
  const data = event.data ?? {};

  return {
    id: nextId(),
    kind: "step",
    icon: event.icon ?? "",
    message: event.message ?? "",
    ...(event.value ? { value: event.value } : {}),
    ...(data.action ? { action: data.action } : {}),
    ...(data.status ? { status: data.status } : {}),
    table: data.table ?? "",
  };
}

/** Сумма счётчиков итога. Ноль — работы над объектами не было. */
export function summaryCount(summary: Summary | null): number {
  if (!summary) return 0;

  return Object.values(summary).reduce((total, value) => total + (value ?? 0), 0);
}

/**
 * Была ли это работа, а не разговор.
 *
 * Простой ответ («да, у product уже есть цена») даёт ровно те же события
 * потока: провайдер, мысль вслух, пустой итог. Рисовать под ним карточку
 * шагов не о чем — там будет одна строка «Анализирую запрос».
 */
export function hasWork(run: Run): boolean {
  return (
    run.active ||
    run.stopped ||
    Boolean(run.error) ||
    summaryCount(run.summary) > 0 ||
    run.steps.some((step) => step.kind === "step")
  );
}
