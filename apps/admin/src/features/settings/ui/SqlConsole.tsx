import { useDeferredValue, useLayoutEffect, useMemo, useRef, useState, type RefObject } from "react";
import {
  IconCopy,
  IconDeviceFloppy,
  IconLoader2,
  IconPlayerPlay,
  IconPlus,
  IconTrash,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { useTables } from "@/features/table";
import { errorMessage, toast } from "@/shared/lib/toast";
import { Button } from "@/shared/ui/button";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Icon } from "@/shared/ui/icon";
import { Field, Input } from "@/shared/ui/input";
import { Modal } from "@/shared/ui/modal";
import {
  useDeleteQuery,
  useRunSql,
  useSaveQuery,
  useSavedQueries,
  useUpdateQuery,
  type SavedQuery,
  type SqlResult,
} from "../api/sql";
import { sqlCell, type SqlCellKind } from "../model/sql-cell";
import {
  sqlGhost,
  sqlParams,
  sqlTablePrefix,
  tokenizeSql,
  type SqlTokenKind,
} from "../model/sql-syntax";
import { Empty, SectionHeader, Td, Th } from "./parts";

/**
 * SQL-консоль: запрос сверху, ответ снизу.
 *
 * Это единственное место в админке, где к базе проекта обращаются
 * напрямую, минуя таблицы и view. Нужна она там, где схема админки
 * ответить не может: сверить данные, посчитать разом по нескольким
 * таблицам, поправить строку, до которой нет экрана.
 *
 * Редактора с подсветкой здесь нет намеренно. У ugen в этом месте
 * Monaco — ради автодополнения по слагам таблиц, — но это редактор кода
 * целиком (мегабайты, свои воркеры, своя тема под каждую нашу), и
 * ставить его ради поля, в которое чаще всего вставляют готовый запрос,
 * дорого. Поле растягивается мышью — это `resize-y`, поведение самого
 * браузера.
 *
 * Ctrl/Cmd+Enter запускает: запрос правят и запускают подряд десятки
 * раз, и тянуться к кнопке на каждый прогон — это и есть работа
 * в консоли.
 *
 * Слева — сохранённые запросы (`api/sql`, SavedQuery). Разобранный
 * однажды перекос в данных спрашивают и через месяц, а держать такой
 * запрос в личных заметках значит, что у соседа его нет.
 */
export function SqlConsole() {
  const { t } = useTranslation();
  const run = useRunSql();
  const save = useSaveQuery();
  const update = useUpdateQuery();

  /* Поле живёт в `Editor`, а ссылка на него здесь: возвращать в него
     курсор решает тот, кто переключает запросы, а не само поле. */
  const field = useRef<HTMLTextAreaElement>(null);

  const [sql, setSql] = useState("");
  /**
   * Открытый сохранённый запрос целиком, а не один его id: по тексту
   * видно, правили ли его с тех пор, как открыли (`dirty` ниже).
   */
  const [opened, setOpened] = useState<SavedQuery | null>(null);
  const [naming, setNaming] = useState(false);
  /** Куда уйти, когда в поле есть несохранённое. */
  const [leaving, setLeaving] = useState<SavedQuery | "new" | null>(null);

  const submit = () => {
    const query = sql.trim();
    if (query && !run.isPending) run.mutate(query);
  };

  /** Написанное отличается от того, что лежит в базе (или от пустоты). */
  const dirty = Boolean(sql.trim()) && sql !== (opened?.sql ?? "");

  const go = (next: SavedQuery | "new") => {
    setSql(next === "new" ? "" : next.sql);
    setOpened(next === "new" ? null : next);
    /* Ответ прошлого запроса под новым текстом — враньё: строки
       от одного запроса, заголовок экрана от другого. */
    run.reset();
    /* Курсор возвращается в поле: и «новый запрос», и открытие
       сохранённого — это выбор того, что сейчас будут править,
       а не конец действия. Иначе первое нажатие клавиши уходит
       в никуда, а после диалога фокус и вовсе остаётся на кнопке,
       которой больше нет. */
    field.current?.focus();
  };

  /** Переход, который затрёт поле: сначала спросить, потом уходить. */
  const leave = (next: SavedQuery | "new") => (dirty ? setLeaving(next) : go(next));

  const saving = save.isPending || update.isPending;

  const store = () => {
    if (saving || !dirty) return;

    if (opened) {
      update.mutate(
        { id: opened.id, sql },
        // Сохранённое и есть новая точка отсчёта: иначе запрос
        // остаётся «изменённым» и после того, как его записали.
        { onSuccess: () => setOpened({ ...opened, sql }) },
      );
      return;
    }

    setNaming(true);
  };

  /* Параметры считаются по тому, что в поле СЕЙЧАС: предупреждение
     нужно и тому, кто их пишет, и тому, кто открыл чужой запрос. */
  const params = sqlParams(sql);

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SectionHeader title={t("sql.title")} hint={t("sql.hint")}>
        <span className="text-2xs text-fg-subtle">{t("sql.shortcut")}</span>

        {/* Гаснет, когда сохранять нечего: у открытого запроса — пока
            его не тронули, у нового — пока поле пусто. Заодно это
            единственный видимый признак, что правка не записана. */}
        <Button size="sm" variant="ghost" disabled={saving || !dirty} onClick={store}>
          <Icon
            as={saving ? IconLoader2 : IconDeviceFloppy}
            size={14}
            className={saving ? "animate-spin" : ""}
          />
          {t("action.save")}
        </Button>

        <Button size="sm" disabled={run.isPending || !sql.trim()} onClick={submit}>
          <Icon
            as={run.isPending ? IconLoader2 : IconPlayerPlay}
            size={14}
            className={run.isPending ? "animate-spin" : ""}
          />
          {run.isPending ? t("common.loading") : t("sql.run")}
        </Button>
      </SectionHeader>

      <div className="flex min-h-0 flex-1">
        <SavedQueries
          openId={opened?.id ?? ""}
          onOpen={leave}
          onNew={() => leave("new")}
          /* Удалили открытый — текст в поле остаётся, связь обрывается:
             следующее «Сохранить» заведёт новый запрос, а не постучится
             по мёртвому идентификатору. */
          onForget={() => setOpened(null)}
        />

        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          <Editor value={sql} field={field} onChange={setSql} onRun={submit} onSave={store} />

          {/* Параметры подставляет только вызов сохранённого запроса
              снаружи; здесь, в `exec-query`, postgres спотыкается
              о двоеточие, а текст его ошибки шлюз выбрасывает — без
              этой строки прогон выглядел бы как «пусто».

              Полоса, а не карточка с отступом: это постоянное свойство
              написанного, а не событие. Оформление — то же, что
              у такой же полосы в ресурсах (`ResourceSettings`). */}
          {params.length > 0 && (
            <p className="shrink-0 border-b border-border bg-warning-subtle px-4 py-2 text-xs text-warning">
              {t("sql.paramsHint", { names: params.join(", ") })}
            </p>
          )}

          {run.isError && (
            <p
              role="alert"
              className="m-4 rounded-md bg-danger-subtle px-3 py-2 text-xs text-danger"
            >
              {errorMessage(run.error, "sql.failed")}
            </p>
          )}

          {!run.isError && !run.data && (
            <p className="p-4 text-sm text-fg-subtle">{t("sql.idle")}</p>
          )}

          {!run.isError && run.data && <Result result={run.data} />}
        </div>
      </div>

      {naming && (
        <NameDialog
          sql={sql}
          busy={save.isPending}
          onClose={() => setNaming(false)}
          onSave={(name) =>
            save.mutate(
              { name, sql },
              {
                onSuccess: (query) => {
                  setOpened(query);
                  setNaming(false);
                  // Диалог закрылся — курсор обратно в поле, а не в никуда.
                  field.current?.focus();
                },
              },
            )
          }
        />
      )}

      {/* Единственное место, где написанное можно потерять: открыть
          соседний запрос значит затереть поле, а отменить это нечем. */}
      {leaving && (
        <ConfirmDialog
          title={t("sql.discardTitle")}
          description={t("sql.discardDescription")}
          confirmLabel={t("sql.discard")}
          busy={false}
          onClose={() => setLeaving(null)}
          onConfirm={() => {
            go(leaving);
            setLeaving(null);
          }}
        />
      )}
    </div>
  );
}

/**
 * Сохранённые запросы слева.
 *
 * Поиска нет: их единицы, а не десятки, и пустое поле поиска над
 * списком из четырёх строк — это вопрос «а где остальные?». Появятся
 * десятки — появится и поле, как у подключений.
 */
function SavedQueries({
  openId,
  onOpen,
  onNew,
  onForget,
}: {
  openId: string;
  onOpen: (query: SavedQuery) => void;
  onNew: () => void;
  /** Открытый запрос удалён: текст оставить, связь с ним оборвать. */
  onForget: () => void;
}) {
  const { t } = useTranslation();
  const { queries, isLoading, error } = useSavedQueries();
  const remove = useDeleteQuery();

  const [deleting, setDeleting] = useState<SavedQuery | null>(null);

  return (
    <nav className="flex w-56 shrink-0 flex-col gap-1 overflow-y-auto border-r border-border p-2">
      <button
        type="button"
        onClick={onNew}
        className="flex h-8 shrink-0 items-center gap-2 rounded-md px-2 text-left text-sm text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
      >
        <Icon as={IconPlus} size={14} className="shrink-0" />
        {t("sql.new")}
      </button>

      {isLoading && <p className="px-2 py-1 text-xs text-fg-subtle">{t("common.loading")}</p>}
      {!isLoading && !queries.length && (
        <p className={`px-2 py-1 text-xs ${error ? "text-danger" : "text-fg-subtle"}`}>
          {error ?? t("sql.noSaved")}
        </p>
      )}

      {queries.map((query) => (
        /* Строка — не кнопка внутри кнопки: открывает одна, удаляет
           другая, и вложить вторую в первую нельзя. */
        <div
          key={query.id}
          className={`group/query flex h-8 items-center rounded-md transition-colors ${
            query.id === openId ? "bg-surface-active text-fg" : "text-fg-muted hover:bg-surface-hover"
          }`}
        >
          <button
            type="button"
            onClick={() => onOpen(query)}
            title={query.name}
            aria-current={query.id === openId ? "true" : undefined}
            className="min-w-0 flex-1 truncate px-2 text-left text-sm hover:text-fg"
          >
            {query.name}
          </button>

          {/*
            Прозрачностью, а не `display`: скрытую кнопку не берёт
            ни Tab, ни `focus-visible`, и удалить запрос с клавиатуры
            было бы нечем. Так же спрятаны кнопки строк в остальных
            разделах настроек (`ApiKeySettings`, `EndpointSettings`).
            `group-focus-within` — чтобы она проявлялась, когда на самой
            строке уже стоит фокус, а не только под курсором.
          */}
          <button
            type="button"
            onClick={() => setDeleting(query)}
            aria-label={t("action.delete")}
            title={t("action.delete")}
            className="mr-1 grid size-6 shrink-0 place-items-center rounded text-fg-subtle opacity-0 transition group-hover/query:opacity-100 group-focus-within/query:opacity-100 hover:bg-danger-subtle hover:text-danger focus-visible:opacity-100"
          >
            <Icon as={IconTrash} size={13} />
          </button>
        </div>
      ))}

      {deleting && (
        <ConfirmDialog
          title={t("sql.deleteTitle", { name: deleting.name })}
          description={t("sql.deleteDescription")}
          confirmLabel={t("action.delete")}
          busy={remove.isPending}
          onClose={() => setDeleting(null)}
          onConfirm={() =>
            remove.mutate(deleting.id, {
              onSuccess: () => {
                if (deleting.id === openId) onForget();
                setDeleting(null);
              },
            })
          }
        />
      )}
    </nav>
  );
}

/**
 * Имя для нового запроса — единственное, что спрашиваем: почему у
 * сохранённого запроса нет ни метода, ни галки транзакции, написано
 * в `api/sql` (`useSaveQuery`), и причина одна — их не читает бэкенд.
 */
function NameDialog({
  sql,
  busy,
  onClose,
  onSave,
}: {
  sql: string;
  busy: boolean;
  onClose: () => void;
  onSave: (name: string) => void;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState("");

  const params = sqlParams(sql);

  return (
    <Modal onClose={onClose}>
      <form
        className="flex w-full max-w-md flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal"
        onSubmit={(event) => {
          event.preventDefault();
          onSave(name);
        }}
      >
        <h2 className="text-base font-semibold">{t("sql.saveTitle")}</h2>

        <Field label={t("sql.name")} hint={t("sql.nameHint")}>
          <Input autoFocus required value={name} onChange={(event) => setName(event.target.value)} />
        </Field>

        {/* Что станет параметром вызова — видно до сохранения, а не
            после первого запроса снаружи. Список тот же наивный, что
            и у бэкенда: `id::text` попадёт сюда как `text`, и это
            единственное место, где такую опечатку заметят. */}
        {params.length > 0 && (
          <p className="text-xs text-fg-muted">
            {t("sql.paramsFound", { names: params.join(", ") })}
          </p>
        )}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button type="submit" disabled={busy || !name.trim()}>
            {busy && <Icon as={IconLoader2} size={14} className="animate-spin" />}
            {busy ? t("common.saving") : t("action.save")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

/**
 * Цвет лексемы — токенами, то есть одинаково читается в обеих темах.
 *
 * Комментарий приглушён до `fg-muted`, а не до `fg-subtle`: последний
 * в системе означает плейсхолдер и «пусто» (app/styles.css), и на
 * подложке поля даёт 2.8:1 при нужных 4.5. Комментарий в запросе —
 * это текст, который читают, а не серое место под ним; `fg-muted`
 * даёт 5.1:1 и остаётся самым тихим цветом в поле. От знаков
 * препинания его отличает курсив, а не оттенок.
 */
const TOKEN_COLOR: Record<SqlTokenKind, string> = {
  keyword: "text-accent-text",
  string: "text-success",
  number: "text-warning",
  comment: "text-fg-muted italic",
  punct: "text-fg-muted",
  plain: "",
};

/**
 * Одинаковые у поля и подложки: любое расхождение — сдвиг текста.
 * `scrollbar-gutter` в том числе: полоса прокрутки, появившаяся у поля
 * и не появившаяся у подложки, сужает строку на свою ширину, и длинные
 * строки начинают переноситься в разных местах.
 */
const EDITOR_TEXT =
  "p-3 font-mono text-xs leading-5 whitespace-pre-wrap break-words [scrollbar-gutter:stable]";

/**
 * Поле запроса с подсветкой и подсказкой.
 *
 * Подложка — `bg-surface`, как у любого поля ввода в системе
 * (`shared/ui/input`), а не фон приложения: на сером `bg-surface`
 * зелёный строкового литерала даёт 4.16:1 при нужных 4.5 — тот же
 * токен на белом проходит. Цвет фона у поля с крашеным текстом — часть
 * контраста, а не оформление.
 *
 * Подсветка — крашеная КОПИЯ текста под прозрачным полем ввода: раскрасить
 * содержимое самого `<textarea>` нельзя, там один цвет на всё. Значит копия
 * обязана лежать пиксель в пиксель — отсюда общий `EDITOR_TEXT` на обоих
 * и перенос прокрутки на подложку. Ради этого же `tokenizeSql` возвращает
 * и пробелы: пропавший пробел сдвинул бы всю строку.
 *
 * Подсказка — серый хвост слова прямо за курсором, а не список: список
 * нужно куда-то поместить (координаты курсора в текстовом поле браузер
 * не отдаёт), в нём нужно ходить стрелками и закрывать его Esc. Хвост
 * даёт то же самое — видно, что допишется, — ценой одной строки разметки.
 *
 * Хвост рисуется пролезающим наружу из нулевой ширины: так следующий
 * текст не сдвигается. Поэтому же подсказки нет, когда дальше по строке
 * что-то есть — она легла бы поверх чужого текста (см. `sqlGhost`).
 */
function Editor({
  value,
  field,
  onChange,
  onRun,
  onSave,
}: {
  value: string;
  /** Ссылка на поле — снаружи: в него возвращают курсор при переключении. */
  field: RefObject<HTMLTextAreaElement | null>;
  onChange: (value: string) => void;
  onRun: () => void;
  onSave: () => void;
}) {
  const { t } = useTranslation();
  const mirror = useRef<HTMLPreElement>(null);
  const [caret, setCaret] = useState(0);

  const tokens = useMemo(() => tokenizeSql(value), [value]);

  /*
   * Имена таблиц — из того же постраничного поиска, что и везде
   * (`useTables`): он ищет по всем таблицам проекта, а не по первой
   * загруженной странице, и его ответы кэшируются на пять минут.
   *
   * Спрашивается только после `from`, `join`, `into`, `update`, `table`
   * и только со второй буквы: в остальных случаях `sqlTablePrefix`
   * отдаёт пусто, и запрос выключен целиком — иначе открытие консоли
   * тянуло бы первую страницу таблиц, которую никто не прочитает.
   * Отложенное значение добавляет то же самое во времени: пока букву
   * дописывают, промежуточные слова до сервера не доезжают.
   */
  const prefix = useDeferredValue(sqlTablePrefix(value, caret));
  const tables = useTables(prefix, Boolean(prefix));
  const slugs = useMemo(() => tables.items.map((table) => table.slug), [tables.items]);

  const ghost = useMemo(() => sqlGhost(value, caret, slugs), [value, caret, slugs]);

  /*
   * Куда поставить курсор после вставки хвоста. Через слой, а не сразу:
   * поле управляемое, и до того, как React запишет в него новый текст,
   * ставить в нём позицию некуда.
   *
   * Слой именно такой — эффект после отрисовки, а не rAF: rAF срабатывает
   * ПОСЛЕ следующего кадра, то есть позже нескольких быстро набранных
   * букв, и отдёргивает курсор назад, к месту подсказки. На синтетическом
   * вводе это видно сразу, у быстро печатающего человека — тоже.
   */
  const pending = useRef<number | null>(null);

  useLayoutEffect(() => {
    if (pending.current === null) return;
    field.current?.setSelectionRange(pending.current, pending.current);
    pending.current = null;
  });

  /** Принять подсказку: дописать хвост и встать за ним. */
  const accept = () => {
    const position = caret + ghost.length;
    onChange(value.slice(0, caret) + ghost + value.slice(caret));
    setCaret(position);
    pending.current = position;
  };

  return (
    /* Тянется мышью сама обёртка, а не поле: поле лежит в ней абсолютно
       и растягивается следом. `resize` — это браузер, не наш код.

       Рамка фокуса — на обёртке, а не на поле: у поля `outline-none`
       (без него браузер обвёл бы прозрачный текст поверх крашеной
       подложки), и общее правило `:focus-visible` из app/styles.css
       до него не доходит — то есть с клавиатуры было не видно, где
       ты находишься. Кольцо на обёртке `overflow-hidden` не срезает:
       оно рисуется снаружи её границы.

       Толщина и цвет — те же 2px акцента, что у общего правила
       `:focus-visible`: фокус во всём приложении выглядит одинаково,
       а не «почти так же». По `focus-within`, а не `focus-visible`:
       в поле ввода фокус показывают и от мыши тоже — так же, как
       `Input` красит границу на обычный `focus`. */
    <div className="relative h-40 shrink-0 resize-y overflow-hidden border-b border-border bg-surface ring-2 ring-transparent transition-shadow focus-within:ring-accent">
      <pre
        ref={mirror}
        aria-hidden
        className={`pointer-events-none absolute inset-0 overflow-hidden text-fg ${EDITOR_TEXT}`}
      >
        {tokens.map((token) => (
          <span key={token.start} className={TOKEN_COLOR[token.kind]}>
            {token.text}
          </span>
        ))}

        {/* Нулевая ширина: хвост виден, а текст за ним не двигается. */}
        {ghost && (
          <span className="inline-block w-0 overflow-visible whitespace-pre text-fg-subtle">
            {ghost}
          </span>
        )}

        {/* Пустая строка в конце: без неё подложка короче поля на строку. */}
        {"\n"}
      </pre>

      <textarea
        ref={field}
        /* Раздел открывают, чтобы написать запрос, — курсор уже здесь,
           и первое нажатие клавиши попадает в поле, а не в пустоту.
           Раздел монтируется только выбранным (`SettingsDialog`),
           так что фокус не уводится ни у кого за спиной. */
        autoFocus
        value={value}
        onChange={(event) => {
          onChange(event.target.value);
          setCaret(event.target.selectionStart);
        }}
        onSelect={(event) => setCaret(event.currentTarget.selectionStart)}
        onScroll={(event) => {
          const pre = mirror.current;
          if (!pre) return;
          pre.scrollTop = event.currentTarget.scrollTop;
          pre.scrollLeft = event.currentTarget.scrollLeft;
        }}
        onKeyDown={(event) => {
          if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
            event.preventDefault();
            onRun();
            return;
          }

          /* Ctrl/Cmd+S — рефлекс любого, кто печатает в поле, похожее
             на редактор. Без перехвата браузер предлагает сохранить
             СТРАНИЦУ, то есть отвечает на нажатие не тем и не туда. */
          if (event.key.toLowerCase() === "s" && (event.metaKey || event.ctrlKey)) {
            event.preventDefault();
            onSave();
            return;
          }

          /* Tab перехватывается ТОЛЬКО при живой подсказке: в остальное
             время это переход по элементам, и отнимать его у клавиатуры
             нельзя — из поля стало бы не выйти. */
          if (event.key === "Tab" && ghost) {
            event.preventDefault();
            accept();
          }
        }}
        spellCheck={false}
        placeholder={t("sql.placeholder")}
        aria-label={t("sql.title")}
        /* Текст прозрачный — виден только крашеный из подложки, — а курсор
           и выделение остаются свои: их подложка нарисовать не может. */
        className={`absolute inset-0 resize-none bg-transparent text-transparent caret-fg outline-none placeholder:text-fg-subtle ${EDITOR_TEXT}`}
      />
    </div>
  );
}

/**
 * Цвет и выключка значения — по тому, что реально приехало (`sqlCell`).
 * Палитра сознательно небогатая: в приложении четыре небейтральных
 * смысловых тона (accent/success/warning/danger — `app/styles.css`),
 * и danger не отдан сюда — ложные данные не ошибка интерфейса. Три
 * оставшихся уже разобраны, четвёртого не будет: пятый цвет в таблице,
 * где и так пять-шесть колонок, начал бы значить не «тип», а «пестро».
 *
 * `null` — тем же цветом, что и слово `null` в самом запросе: оно есть
 * в списке ключевых слов (`sql-syntax.ts`, `TOKEN_COLOR.keyword` выше)
 * и красится в `text-accent-text` уже там. Одна и та же отметка одним
 * и тем же цветом — что написали, то и увидели в ответе.
 *
 * `true`/`false` — тем же приёмом, что уже есть в двух других местах
 * репозитория (`RegisterForm`, `ItemDrawer`): выполнено — success,
 * не выполнено — приглушённый текст, а не danger. `false` не ошибка.
 *
 * `number` — тем же тоном, что числа-литералы в самом запросе
 * (`TOKEN_COLOR.number`): третий и последний свободный смысловой цвет,
 * и та же логика, что у `null`, — то же самое видно тут и там.
 *
 * `uuid`/`date` цвета не получают вовсе — не от нехватки тона,
 * а по смыслу: это не данные, ради которых пишут запрос, а служебная
 * привязка (id) и почти всегда второстепенное поле (created_at рядом
 * с настоящими значениями строки). Их приглушают, а не выделяют — тем
 * же приёмом, что уже стоит в `ActivityLog`/`FunctionLogs` на датах
 * и в половине настроек на технических строках (путь, слаг, токен):
 * `font-mono text-fg-muted`. Выделить цветом здесь значило бы обратное
 * тому, что нужно, — тянуть взгляд туда, где смотреть не за чем.
 */
const CELL_STYLE: Record<SqlCellKind, string> = {
  null: "text-accent-text italic",
  boolean: "",
  number: "text-warning tabular-nums",
  uuid: "text-fg-muted",
  date: "text-fg-muted tabular-nums",
  text: "",
};

const CELL_TRUE = "text-success";
/*
 * `false` приглушён до `fg-muted`, а не до `fg-subtle`, как невыполненное
 * правило пароля в `RegisterForm`: там серым помечено СОСТОЯНИЕ, которого
 * ещё нет, а здесь это значение в ячейке — то самое, ради чего запрос
 * и писали. Цена разницы — 2.8:1 против 5.1:1 на белом.
 */
const CELL_FALSE = "text-fg-muted";

/** Ответ: таблица, счётчик или объяснение, почему показать нечего. */
function Result({ result }: { result: SqlResult }) {
  const { t } = useTranslation();

  /*
   * Ни колонок, ни строк — и это ровно тот случай, о котором сказать
   * нечего: текст ошибки SQL шлюз выбрасывает (api/sql.ts), поэтому
   * упавший запрос выглядит так же, как выполненный DDL. Пишем обе
   * возможности словами, а не показываем бодрое «Готово» там, где мы
   * не знаем, готово ли.
   */
  if (!result.columns.length) {
    return (
      <p className="p-4 text-sm text-fg-muted">
        {result.rowsAffected > 0
          ? t("sql.affected", { count: result.rowsAffected })
          : t("sql.blank")}
      </p>
    );
  }

  return (
    <>
      <div className="min-h-0 flex-1 overflow-auto">
        {/*
          Без `w-full`: таблица растёт до своего содержимого, а не до
          ширины панели. При двух-трёх узких колонках `w-full` тянул бы
          заголовок и разделители во всю ширину экрана при обрезанном
          по-настоящему контенте (см. комментарий у `max-w-xs` ниже) —
          пустая полоса после текста выглядела бы как недоделанная
          вёрстка. Когда колонок много и им тесно, таблица всё равно
          не сожмётся уже своего содержимого — за прокрутку отвечает
          обёртка с `overflow-auto`, ей ширина таблицы не указ.
        */}
        <table className="border-separate border-spacing-0">
          <thead>
            <tr>
              {result.columns.map((column) => (
                <Th key={column}>
                  <span className="font-mono text-fg">{column}</span>
                  {/* Тип колонки — от базы, а не от значения: по нему
                      видно, что `1` в этой колонке int4, а не text. */}
                  {result.types[column] && (
                    <span className="ml-1.5 font-mono text-2xs text-fg-subtle">
                      {result.types[column]}
                    </span>
                  )}
                </Th>
              ))}
            </tr>
          </thead>

          <tbody>
            {!result.rows.length && <Empty text={t("sql.noRows")} colSpan={result.columns.length} />}

            {result.rows.map((row, index) => (
              // Ключ по номеру: своего идентификатора у строки
              // произвольного запроса нет — его может не быть в выборке.
              // group/row — под ним прячется кнопка копирования ячейки:
              // семь десятков кнопок, видных разом, шумят больше, чем
              // помогают, а на каждую строку они и не нужны одновременно.
              <tr key={index} className="group/row hover:bg-surface-hover">
                {result.columns.map((column) => {
                  const { text, kind } = sqlCell(row[column], result.types[column]);
                  const boolColor = kind === "boolean" ? (text === "true" ? CELL_TRUE : CELL_FALSE) : "";

                  return (
                    // relative — под кнопку копирования: у неё absolute,
                    // и без этого она искала бы предка выше по дереву,
                    // а не эту ячейку.
                    <Td key={column} className="relative font-mono text-xs">
                      {/*
                        Своя ширина у текста, а не ширина ячейки: под
                        `table-layout: auto` (по умолчанию) `max-width`
                        на самой `<td>` — не граница, а подсказка алгоритму
                        раскладки, и он вправе её превысить, если у таблицы
                        есть свободное место. При двух-трёх колонках места
                        хватает всегда, и обрезка не срабатывала бы вовсе.
                        Ограничение здесь, на обычном блочном элементе, —
                        настоящий потолок независимо от того, сколько места
                        досталось ячейке. `ml-auto` у числа — тот же приём,
                        что раньше делал `justify-end`, но без flex: блок
                        уже своей ширины, просто прижатый к правому краю.
                      */}
                      <span
                        className={`block max-w-xs truncate ${kind === "number" ? "ml-auto" : ""} ${CELL_STYLE[kind]} ${boolColor}`}
                        title={kind === "null" ? undefined : text}
                      >
                        {text}
                      </span>

                      {/*
                        Кнопка копирования — absolute, не участник потока:
                        появление меняет только её видимость, а не место
                        соседей. Была бы она обычным элементом строки, её
                        появление раздвигало бы текст и остальные ячейки
                        в момент наведения — дёрганье, которое заметно
                        на каждой строке, а не польза. Копируется одна
                        ячейка, не вся строка: из ответа произвольного
                        запроса нужно обычно одно значение, чаще всего id,
                        чтобы вставить его в другой запрос.

                        Прячется прозрачностью, а не `display`: скрытую
                        `display: none` кнопку не берёт ни Tab, ни
                        `focus-visible`, то есть скопировать значение
                        с клавиатуры было бы нельзя вовсе.
                      */}
                      <button
                        type="button"
                        onClick={() => {
                          void navigator.clipboard.writeText(text);
                          toast.success(t("cell.copied"));
                        }}
                        aria-label={t("cell.copy")}
                        title={t("cell.copy")}
                        className="absolute top-1/2 right-1 grid size-5 -translate-y-1/2 place-items-center rounded bg-surface text-fg-muted opacity-0 shadow-raised transition group-hover/row:opacity-100 hover:bg-surface-active hover:text-fg focus-visible:opacity-100"
                      >
                        <Icon as={IconCopy} size={12} />
                      </button>
                    </Td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex h-9 shrink-0 items-center border-t border-border px-4 text-2xs text-fg-subtle">
        {t("sql.rows", { count: result.rows.length })}
      </div>
    </>
  );
}
