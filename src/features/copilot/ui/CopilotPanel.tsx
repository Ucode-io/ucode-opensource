import { useEffect, useRef, useState, type RefObject } from "react";
import {
  IconArrowUp,
  IconPlayerStopFilled,
  IconDatabase,
  IconFolder,
  IconHistory,
  IconLink,
  IconMessage,
  IconPlus,
  IconSearch,
  IconSparkles,
  IconTable,
  IconX,
  type Icon as TablerIcon,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { COPILOT_MAX_WIDTH, COPILOT_MIN_WIDTH, useUi } from "@/shared/lib/ui-store";
import type { TranslationKey } from "@/shared/lib/i18n";
import { Icon } from "@/shared/ui/icon";
import { Popover } from "@/shared/ui/popover";
import { ResizeHandle } from "@/shared/ui/resize-handle";
import { useChatSessions } from "../api/chat";
import { useCopilotChat } from "../model/use-chat";
import { Conversation } from "./Conversation";

/**
 * Помощник u-code: беседа, из которой заводятся таблицы, поля, связи
 * и разделы меню.
 *
 * Панель, а не модальное окно: работа идёт над тем, что на экране,
 * и просьба «добавь сюда цену» имеет смысл, только пока таблица видна.
 * Поэтому она отодвигает контент, а не накрывает его.
 *
 * Смонтирована всегда — рисует пусто, когда закрыта. Иначе закрытие
 * панели теряло бы недописанный разговор вместе с её состоянием.
 */
export function CopilotPanel() {
  const { t } = useTranslation();
  const { copilotOpen, copilotWidth, setCopilotWidth, closeCopilot, sidebarCollapsed } = useUi();
  const chat = useCopilotChat();

  const panel = useRef<HTMLElement>(null);
  /*
   * Поле ввода наружу: панель открывают, чтобы что-то спросить, —
   * курсор обязан оказаться в нём сам. И вернуться туда же после
   * нажатия на подсказку: она кладёт текст, который дописывают.
   */
  const box = useRef<HTMLTextAreaElement>(null);
  const [input, setInput] = useState("");

  /*
   * Курсор в поле — на КАЖДОЕ открытие, а не на монтирование: панель
   * с экрана не снимается (иначе ей нечем уезжать), и `autoFocus`
   * сработал бы один раз за жизнь вкладки — при загрузке приложения,
   * в закрытую панель.
   */
  useEffect(() => {
    if (copilotOpen) box.current?.focus();
  }, [copilotOpen]);

  const submit = (text: string) => {
    if (!text.trim() || chat.sending) return;

    void chat.send(text);
    setInput("");
  };

  const startOver = () => {
    chat.reset();
    setInput("");
    box.current?.focus();
  };

  const fill = (prompt: string) => {
    setInput(prompt);
    box.current?.focus();
  };

  return (
    <aside
      ref={panel}
      /*
       * Закрытая уезжает ОТРИЦАТЕЛЬНЫМ ОТСТУПОМ, как и сайдбар: ширину
       * в это же время пишет ручка размера, и переход по ней превратил
       * бы перетаскивание в желе. Отступ двигает и панель, и контент
       * за ней — одним свойством.
       *
       * Уехавшая лежит за правым краем окна; чтобы она не завела там
       * горизонтальную прокрутку, оболочка приложения обрезает по X
       * (см. _authed.tsx).
       */
      style={{ width: copilotWidth, marginRight: copilotOpen ? 0 : -copilotWidth }}
      /* За краем экрана, но в DOM — значит, вне обхода с клавиатуры:
         Tab не должен уводить в невидимое. */
      inert={!copilotOpen}
      /* Панель — это фон приложения, а не карточка на нём: ни рамки,
         ни скругления, ни полей, как у сайдбара. Карточка здесь одна —
         контент; всё, что от неё справа и слева, лежит на общем фоне.
         Белым в панели остаётся только то, что «висит»: лента шагов
         и поле ввода. */
      /* Отступ сверху — тот же, что у карточки контента (m-2) и сайдбара
         (p-2): иначе шапка помощника висит на 8px выше шапки страницы,
         и три заголовка на экране стоят на двух разных уровнях. Карточки
         нет — нет и отступа. */
      className={`relative flex shrink-0 flex-col overflow-hidden transition-[margin-right] duration-200 ease-out ${
        sidebarCollapsed ? "" : "pt-2"
      }`}
    >
      <ResizeHandle
        edge="left"
        target={panel}
        value={copilotWidth}
        min={COPILOT_MIN_WIDTH}
        max={COPILOT_MAX_WIDTH}
        label={t("copilot.resize")}
        onCommit={setCopilotWidth}
      />

      {/* Шапка без разделительной линии: делить нечего — под ней тот же
          фон, а не другая поверхность. */}
      <header className="flex h-header shrink-0 items-center gap-2 px-3">
        <span className="grid size-6 shrink-0 place-items-center rounded-md bg-accent-subtle text-accent-text">
          <Icon as={IconSparkles} size={14} />
        </span>
        <span className="truncate text-sm font-medium" title={chat.title}>
          {chat.title ? firstLine(chat.title) : t("copilot.newChat")}
        </span>

        <div className="ml-auto flex items-center gap-0.5">
          <Popover
            align="end"
            trigger={({ toggle }) => (
              <HeaderButton icon={IconHistory} label={t("copilot.history")} onClick={toggle} />
            )}
          >
            {/* Список висит внутри поповера, то есть монтируется вместе
                с ним — этим и включается запрос: пока историю не открыли,
                её незачем грузить. */}
            {(close) => <History onPick={chat.open} close={close} />}
          </Popover>

          <HeaderButton icon={IconPlus} label={t("copilot.newChat")} onClick={startOver} />
          <HeaderButton icon={IconX} label={t("action.close")} onClick={closeCopilot} />
        </div>
      </header>

      {chat.messages.length > 0 || chat.run ? (
        <Conversation messages={chat.messages} run={chat.run} />
      ) : (
        <Greeting onPick={fill} />
      )}

      <Composer
        inputRef={box}
        value={input}
        onChange={setInput}
        onSubmit={submit}
        onStop={chat.stop}
        busy={chat.sending}
      />
    </aside>
  );
}

/**
 * Кнопка, открывающая панель. Стоит в шапке страницы, как и в старой
 * админке: помощник — это про то, что сейчас на экране.
 */
export function CopilotButton() {
  const { t } = useTranslation();
  const { copilotOpen, toggleCopilot } = useUi();

  return (
    <button
      type="button"
      onClick={toggleCopilot}
      aria-pressed={copilotOpen}
      title={t("copilot.title")}
      className={`flex h-7 shrink-0 cursor-pointer items-center gap-1.5 rounded-md px-2 text-sm font-medium transition-colors ${
        copilotOpen
          ? "bg-accent-subtle text-accent-text"
          : "text-fg-muted hover:bg-surface-hover hover:text-fg"
      }`}
    >
      <Icon as={IconSparkles} size={16} />
      <span>{t("copilot.title")}</span>
    </button>
  );
}

export function HeaderButton({
  icon,
  label,
  onClick,
}: {
  icon: TablerIcon;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      title={label}
      className="grid size-7 shrink-0 cursor-pointer place-items-center rounded-md text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
    >
      <Icon as={icon} size={16} />
    </button>
  );
}

/**
 * Пустой экран беседы. Подсказки — не украшение: помощник умеет ровно
 * шесть вещей (`ai_ucode_tools.go`), и без примеров человек этого не
 * узнает, а спросит то, чего он не может.
 */
const SUGGESTIONS: { icon: TablerIcon; title: TranslationKey; prompt: TranslationKey }[] = [
  { icon: IconTable, title: "copilot.hint.tableTitle", prompt: "copilot.hint.tablePrompt" },
  { icon: IconLink, title: "copilot.hint.relationTitle", prompt: "copilot.hint.relationPrompt" },
  { icon: IconFolder, title: "copilot.hint.menuTitle", prompt: "copilot.hint.menuPrompt" },
  { icon: IconDatabase, title: "copilot.hint.itemsTitle", prompt: "copilot.hint.itemsPrompt" },
  { icon: IconSearch, title: "copilot.hint.schemaTitle", prompt: "copilot.hint.schemaPrompt" },
];

export function Greeting({ onPick }: { onPick: (prompt: string) => void }) {
  const { t } = useTranslation();

  return (
    /* По центру, а не у нижнего края: пустая беседа — это не начало
       ленты, а заставка. Прижатый к полю ввода текст читался как
       первая реплика, которой нет. */
    <div className="flex flex-1 flex-col items-center justify-center gap-6 overflow-y-auto px-4 py-8">
      <div className="flex flex-col items-center text-center">
        <span className="grid size-11 place-items-center rounded-lg bg-accent-subtle text-accent-text">
          <Icon as={IconSparkles} size={22} />
        </span>
        <div className="mt-3 text-lg font-medium">{t("copilot.greeting")}</div>
        <div className="mt-1 max-w-xs text-sm text-balance text-fg-muted">
          {t("copilot.greetingHint")}
        </div>
      </div>

      <div className="flex w-full max-w-sm flex-col gap-1.5">
        {SUGGESTIONS.map((item) => (
          <button
            key={item.title}
            type="button"
            // Подсказка кладётся в поле ввода, а не отправляется: её
            // почти всегда правят — «товаров» на «заказов».
            onClick={() => onPick(t(item.prompt))}
            className="group flex cursor-pointer items-center gap-2.5 rounded-md bg-surface px-3 py-2.5 text-left shadow-raised transition hover:bg-surface-hover"
          >
            <Icon
              as={item.icon}
              size={16}
              className="shrink-0 text-fg-subtle transition-colors group-hover:text-accent-text"
            />
            <span className="truncate text-sm">{t(item.title)}</span>
            <Icon
              as={IconArrowUp}
              size={14}
              className="ml-auto shrink-0 rotate-45 text-fg-subtle opacity-0 transition-opacity group-hover:opacity-100"
            />
          </button>
        ))}
      </div>
    </div>
  );
}

/** Прошлые беседы. Список запрашивается, только когда меню открыто. */
export function History({
  onPick,
  close,
}: {
  onPick: (chatId: string, title: string) => void;
  close: () => void;
}) {
  const { t } = useTranslation();
  const { sessions, isLoading } = useChatSessions(true);

  if (isLoading) {
    return <div className="px-2 py-3 text-xs text-fg-muted">{t("common.loading")}</div>;
  }

  if (sessions.length === 0) {
    return <div className="px-2 py-3 text-xs text-fg-muted">{t("copilot.noHistory")}</div>;
  }

  return (
    <div className="max-h-80 w-72 overflow-y-auto">
      {sessions.map((session) => (
        <button
          key={session.id}
          type="button"
          role="menuitem"
          onClick={() => {
            void onPick(session.id, session.title);
            close();
          }}
          className="flex w-full cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-surface-hover"
        >
          <Icon as={IconMessage} size={14} className="text-fg-muted" />
          <span className="min-w-0 flex-1 truncate text-sm">
            {firstLine(session.title) || t("copilot.untitled")}
          </span>
          <span className="shrink-0 text-2xs text-fg-subtle">{when(session.updatedAt)}</span>
        </button>
      ))}
    </div>
  );
}

/**
 * Поле ввода.
 *
 * Растёт под текст до половины панели, дальше прокручивается: просьба
 * к помощнику бывает и в пять строк, но поле ввода на весь экран —
 * это уже не переписка.
 */
export function Composer({
  inputRef,
  value,
  onChange,
  onSubmit,
  onStop,
  busy,
}: {
  inputRef: RefObject<HTMLTextAreaElement | null>;
  value: string;
  onChange: (value: string) => void;
  onSubmit: (value: string) => void;
  onStop: () => void;
  busy: boolean;
}) {
  const { t } = useTranslation();

  useEffect(() => {
    const el = inputRef.current;
    if (!el) return;

    el.style.height = "auto";
    el.style.height = `${el.scrollHeight}px`;
  }, [inputRef, value]);

  const placeholder = t("copilot.placeholder");
  const label = busy ? t("copilot.stop") : t("copilot.send");

  return (
    <div className="shrink-0 p-3">
      {/* Островок: поле «висит» над фоном панели тенью, а не обводится
          рамкой. Фокус видно по кольцу — оно не занимает места и не
          двигает содержимое, в отличие от смены цвета рамки. */}
      <div className="flex items-end gap-2 rounded-lg bg-surface p-2 shadow-popover ring-2 ring-transparent transition-shadow focus-within:ring-accent/40">
        <textarea
          ref={inputRef}
          rows={1}
          /* Курсор ставит сама панель, на каждое открытие (см.
             CopilotPanel): она смонтирована всегда, и autoFocus здесь
             сработал бы разово — при загрузке приложения. */
          value={value}
          onChange={(event) => onChange(event.target.value)}
          onKeyDown={(event) => {
            // Enter отправляет, Shift+Enter переносит строку. Пока идёт
            // ответ — не отправляет ничего: вторая реплика встала бы
            // в очередь и потерялась.
            if (event.key !== "Enter" || event.shiftKey) return;

            event.preventDefault();
            onSubmit(value);
          }}
          placeholder={placeholder}
          aria-label={placeholder}
          className="max-h-40 min-w-0 flex-1 resize-none bg-transparent px-1 py-1 text-sm outline-none placeholder:text-fg-subtle"
        />

        {/*
          Одна кнопка на два состояния, а не две рядом: пока идёт ответ,
          отправлять нечего, и единственное осмысленное действие здесь —
          перестать ждать. Место кнопки при этом не меняется.
        */}
        <button
          type="button"
          onClick={() => (busy ? onStop() : onSubmit(value))}
          disabled={!busy && !value.trim()}
          aria-label={label}
          title={label}
          className="grid size-7 shrink-0 cursor-pointer place-items-center rounded-md bg-accent-solid text-accent-fg transition-colors hover:bg-accent-solid-hover disabled:cursor-default disabled:opacity-30"
        >
          <Icon as={busy ? IconPlayerStopFilled : IconArrowUp} size={busy ? 12 : 16} />
        </button>
      </div>
    </div>
  );
}

/** Первая строка заголовка беседы: остальное в кнопку всё равно не влезет. */
const firstLine = (title: string) => title.split("\n")[0]?.trim() ?? "";

/** «17 июн, 14:40» — по локали интерфейса. */
function when(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";

  return date.toLocaleString(undefined, {
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}
