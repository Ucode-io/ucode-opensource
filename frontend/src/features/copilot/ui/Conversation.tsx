import { useEffect, useRef } from "react";
import {
  IconAlertCircle,
  IconAlertTriangle,
  IconBrain,
  IconCircleCheck,
  IconCircleMinus,
  IconCirclePlus,
  IconColumns,
  IconCopy,
  IconCpu,
  IconDatabase,
  IconFolder,
  IconLink,
  IconLoader2,
  IconPlayerStop,
  IconPoint,
  IconSearch,
  IconShield,
  IconSparkles,
  IconUsers,
  type Icon as TablerIcon,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { toast } from "@/shared/lib/toast";
import { Icon } from "@/shared/ui/icon";
import { blocks, inline } from "../model/rich-text";
import { hasWork, summaryCount, type Message, type Run, type Step } from "../model/run";

/**
 * Переписка: реплики человека пузырьками справа, ответы помощника —
 * обычным текстом во всю ширину.
 *
 * Пузырёк только у человека, и это не украшение: его реплики короткие
 * и их надо отличать взглядом, а ответ помощника — это текст, который
 * читают, и рамка вокруг него сужает строку без всякой пользы.
 */
export function Conversation({ messages, run }: { messages: Message[]; run: Run | null }) {
  const end = useRef<HTMLDivElement>(null);
  /**
   * Держаться низа. Пока человек внизу — список едет за ответом; стоит
   * ему отлистать выше, чтобы перечитать шаг, прокрутка отпускает: иначе
   * каждое событие потока (а их десятки) утаскивало бы текст из-под глаз.
   */
  const stick = useRef(true);

  useEffect(() => {
    if (stick.current) end.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [messages, run]);

  return (
    <div
      onScroll={(event) => {
        const { scrollTop, scrollHeight, clientHeight } = event.currentTarget;
        stick.current = scrollHeight - scrollTop - clientHeight < STICKY_ZONE;
      }}
      /* Ответ приходит сам, без действия человека: экранный диктор
         обязан его прочитать. `polite` — дочитав текущую фразу. */
      role="log"
      aria-live="polite"
      className="flex flex-1 flex-col gap-4 overflow-y-auto px-4 py-4"
    >
      {messages.map((message) =>
        message.role === "user" ? (
          <div key={message.id} className="flex justify-end">
            {/* break-words: слаг или ссылка без пробелов иначе растянет
                пузырёк за край панели. */}
            <div className="max-w-[85%] rounded-xl rounded-br-sm bg-surface-hover px-3 py-2 text-sm break-words whitespace-pre-wrap">
              {message.content}
            </div>
          </div>
        ) : (
          <Answer key={message.id} run={message.run ?? null} content={message.content} />
        ),
      )}

      {run && <Answer run={run} content="" />}

      <div ref={end} />
    </div>
  );
}

/** Насколько близко к низу считается «человек внизу», в пикселях. */
const STICKY_ZONE = 80;

/** Ответ помощника: сначала чем он занимался, потом что сказал. */
function Answer({ run, content }: { run: Run | null; content: string }) {
  const { t } = useTranslation();
  const text = run?.content || content;

  return (
    <div className="group/answer flex flex-col gap-2">
      {run && hasWork(run) && <Timeline run={run} />}
      {text && <RichText source={text} />}
      {run?.error && (
        <div className="flex items-start gap-2 rounded-lg bg-danger-subtle px-3 py-2 text-xs text-danger">
          <Icon as={IconAlertCircle} size={14} className="mt-px" />
          <span className="break-words">{run.error}</span>
        </div>
      )}

      {/*
        Ответ уносят наружу — в задачу, в переписку, в описание поля.
        Кнопка появляется по наведению: она нужна изредка, а стоять
        под каждым ответом ей незачем.
      */}
      {text && (
        <button
          type="button"
          onClick={() => {
            void navigator.clipboard.writeText(text);
            toast.success(t("cell.copied"));
          }}
          aria-label={t("cell.copy")}
          title={t("cell.copy")}
          className="grid size-6 cursor-pointer place-items-center self-start rounded-md text-fg-subtle opacity-0 transition hover:bg-surface-hover hover:text-fg group-hover/answer:opacity-100 focus-visible:opacity-100"
        >
          <Icon as={IconCopy} size={14} />
        </button>
      )}
    </div>
  );
}

/**
 * Список шагов сборки.
 *
 * Он не декорация: помощник правит СХЕМУ проекта — заводит таблицы,
 * поля и связи. Человек обязан видеть, что именно изменилось, иначе
 * «Готово!» ничем не отличается от «ничего не сделал».
 */
function Timeline({ run }: { run: Run }) {
  const { t } = useTranslation();

  return (
    // Поверхность, а не фон: панель сама лежит на фоне приложения,
    // и лента шагов на нём читается только как отдельная карточка.
    <div className="flex flex-col gap-1.5 rounded-lg border border-border bg-surface px-2.5 py-2">
      {run.provider && (
        <div className="flex items-center gap-1.5 text-2xs text-fg-subtle">
          <Icon as={IconCpu} size={12} />
          <span className="truncate">{run.provider}</span>
        </div>
      )}

      {run.steps.length === 0 && run.active && (
        <div className="flex items-center gap-2 text-xs text-fg-muted">
          <Icon as={IconSparkles} size={14} className="animate-spin" />
          <span>{t("copilot.thinking")}</span>
        </div>
      )}

      {run.steps.map((step) => (
        <StepRow key={step.id} step={step} />
      ))}

      {/* Оговорка обязательна: остановили ЧТЕНИЕ, а сборка идёт дальше
          на сервере. Без неё «остановлено» читается как «отменено»,
          и человек не поймёт, откуда взялась таблица. */}
      {run.stopped && (
        <div className="flex items-start gap-2 text-2xs text-fg-subtle">
          <Icon as={IconPlayerStop} size={12} className="mt-px" />
          <span>{t("copilot.stopped")}</span>
        </div>
      )}

      {summaryCount(run.summary) > 0 && <Counters run={run} />}
    </div>
  );
}

function StepRow({ step }: { step: Step }) {
  if (step.kind === "reasoning") {
    return (
      <div className="flex items-start gap-2 text-xs text-fg-muted">
        <Icon as={stepIcon(step.icon)} size={14} className="mt-px text-fg-subtle" />
        {/* Мысль вслух пишет та же модель, что и ответ, — с теми же
            обратными кавычками вокруг слагов. */}
        <span>
          <Inline source={step.message} />
        </span>
      </div>
    );
  }

  // Поле, связь и записи — это работа ВНУТРИ таблицы, названной строкой
  // выше. Отступ показывает вложенность без второго уровня разметки.
  const nested = ["field", "relation", "items", "schema"].includes(step.action ?? "");

  return (
    <div className={`flex items-center gap-2 text-xs ${nested ? "pl-4" : ""}`}>
      <Icon as={stepIcon(step.icon)} size={14} className="text-fg-muted" />
      <span className="truncate text-fg">{step.message}</span>
      {step.value && (
        <span className="truncate font-mono text-2xs text-fg-muted">{step.value}</span>
      )}
      <StatusGlyph status={step.status} />
    </div>
  );
}

function StatusGlyph({ status }: { status: string | undefined }) {
  switch (status) {
    case "started":
      return <Icon as={IconLoader2} size={13} className="ml-auto animate-spin text-fg-subtle" />;
    case "done":
      return <Icon as={IconCircleCheck} size={13} className="ml-auto text-success" />;
    case "skipped":
      return <Icon as={IconCircleMinus} size={13} className="ml-auto text-fg-subtle" />;
    case "failed":
      return <Icon as={IconAlertTriangle} size={13} className="ml-auto text-danger" />;
    default:
      return null;
  }
}

/**
 * Итог реплики: «Таблицы 2 · Поля 7 · 4 с».
 *
 * Подпись отдельно от числа, а не «2 таблицы»: согласование по падежу
 * в русском и узбекском стоит трёх ключей на каждый счётчик ради одной
 * цифры. Так же считает и шапка таблицы.
 */
function Counters({ run }: { run: Run }) {
  const { t } = useTranslation();
  const summary = run.summary ?? {};

  const counts = [
    ["copilot.summary.tables", summary.tables],
    ["copilot.summary.fields", summary.fields],
    ["copilot.summary.relations", summary.relations],
    ["copilot.summary.menus", summary.menus],
    ["copilot.summary.items", summary.items],
  ] as const;

  return (
    <div className="mt-0.5 flex flex-wrap gap-x-3 gap-y-1 border-t border-border pt-1.5 text-2xs text-fg-muted">
      {counts.map(([key, value]) =>
        value ? (
          <span key={key}>
            {t(key)} <span className="font-medium text-fg">{value}</span>
          </span>
        ) : null,
      )}
      {run.duration !== null && (
        <span className="ml-auto">
          {run.duration} {t("copilot.summary.seconds")}
        </span>
      )}
    </div>
  );
}

/** Разобранный markdown ответа. Разбор — в model/rich-text. */
export function RichText({ source }: { source: string }) {
  return (
    <div className="flex flex-col gap-2 text-sm leading-relaxed break-words text-fg">
      {blocks(source).map((block, index) =>
        block.kind === "code" ? (
          <pre
            key={index}
            className="overflow-x-auto rounded-md border border-border bg-surface px-2.5 py-2 font-mono text-xs"
          >
            {block.text}
          </pre>
        ) : block.kind === "list" ? (
          <ul key={index} className="flex flex-col gap-1 pl-1">
            {block.items.map((item, at) => (
              <li key={at} className="flex gap-2">
                <span className="text-fg-subtle">{block.ordered ? `${at + 1}.` : "•"}</span>
                <span>
                  <Inline source={item} />
                </span>
              </li>
            ))}
          </ul>
        ) : (
          <p key={index} className="whitespace-pre-wrap">
            <Inline source={block.text} />
          </p>
        ),
      )}
    </div>
  );
}

function Inline({ source }: { source: string }) {
  return inline(source).map((token, index) =>
    token.kind === "bold" ? (
      <strong key={index} className="font-semibold">
        {token.text}
      </strong>
    ) : token.kind === "code" ? (
      <code
        key={index}
        className="rounded-sm bg-surface-hover px-1 py-px font-mono text-xs text-accent-text"
      >
        {token.text}
      </code>
    ) : (
      <span key={index}>{token.text}</span>
    ),
  );
}

/**
 * Значки шагов приходят из потока именами Lucide (`generation_stream.go:40`),
 * а рисуем мы Tabler. Перевод один и здесь; незнакомое имя — точка,
 * а не пустое место: новый шаг на бэкенде не должен ломать список.
 */
const ICONS: Record<string, TablerIcon> = {
  cpu: IconCpu,
  sparkles: IconSparkles,
  brain: IconBrain,
  "scan-search": IconSearch,
  database: IconDatabase,
  columns: IconColumns,
  link: IconLink,
  folder: IconFolder,
  "plus-circle": IconCirclePlus,
  shield: IconShield,
  "shield-check": IconShield,
  users: IconUsers,
  "alert-triangle": IconAlertTriangle,
  "alert-circle": IconAlertCircle,
  "check-circle": IconCircleCheck,
};

const stepIcon = (name: string): TablerIcon => ICONS[name] ?? IconPoint;
