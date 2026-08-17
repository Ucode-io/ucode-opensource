import { useEffect, useMemo, useRef, useState, type DragEvent, type ReactNode } from "react";
import {
  IconCheck,
  IconChevronsRight,
  IconGripVertical,
  IconHeading,
  IconLayoutSidebarRightExpand,
  IconSquare,
  IconSquareToggleHorizontal,
  IconX,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import {
  fieldsForLanguage,
  hasMultilanguage,
  localized,
  baseSlug,
  stripLanguage,
  type Field,
  type Relation,
} from "@/features/table";
import type { DataLanguage } from "@/features/workspace";
import {
  DRAWER_MAX_WIDTH,
  DRAWER_MIN_WIDTH,
  useUi,
  type DrawerMode,
} from "@/shared/lib/ui-store";
import { Icon } from "@/shared/ui/icon";
import { Popover, PopoverItem } from "@/shared/ui/popover";
import { ResizeHandle } from "@/shared/ui/resize-handle";
import { cellKind, editorKind } from "../model/cell-kind";
import type { Item } from "../model/types";
import { Cell } from "./Cell";
import { ActiveCell } from "./CellEditor";
import { fieldIcon } from "./field-icon";

/**
 * Строка целиком, сбоку от таблицы.
 *
 * Таблица показывает строку поперёк: сорок колонок по 180 пикселей,
 * из которых видно четыре. Drawer показывает её вдоль — все поля
 * столбиком, с полным значением в каждом.
 *
 * Правка — теми же редакторами, что и в таблице (ActiveCell): у поля
 * один способ правки, где бы его ни открыли. Иначе получается то, что
 * было в старом ucode: ячейка и форма поддерживают разные наборы типов,
 * и в drawer'е правится то, что в таблице нет, и наоборот.
 *
 * Строка берётся из уже загруженной страницы, а не запрашивается заново:
 * drawer открывают из таблицы, и данные для него уже в кэше.
 *
 * ponytail: ссылку на строку с чужой страницы (или после перезагрузки
 * с другим фильтром) открыть нельзя — строки нет в списке. Лечится
 * запросом одной строки по guid, когда это понадобится.
 */
export function ItemDrawer({
  tableSlug,
  columns,
  row,
  relations,
  locale,
  language,
  languages,
  sections,
  heading,
  tabs,
  tab,
  onTab,
  tabContent,
  onEdit,
  onSettings,
  onReorder,
  onHeading,
  onClose,
}: {
  tableSlug: string;
  /** Поля записи в порядке карточки — своём, не в порядке колонок таблицы. */
  columns: Field[];
  row: Item | undefined;
  relations: Relation[];
  locale: string;
  language: string;
  /**
   * Языки ДАННЫХ проекта. Нужны, чтобы собрать языковые колонки
   * мультиязычного поля обратно в одно — см. features/table/model/multilanguage.
   */
  languages: DataLanguage[];
  /** Секции карточки из раскладки: имя и слаги. Пусто — один общий список. */
  sections: { label: string; slugs: string[] }[];
  /** Слаг поля-заголовка. Пусто — заголовка нет, все поля идут списком. */
  heading: string;
  /**
   * Вкладки связей: связанные строки рядом с карточкой. Их содержимое
   * рисует вызывающий — это чужая таблица со своими колонками, и знать
   * о ней карточка не обязана. Пусто — вкладок нет, показана только карточка.
   */
  tabs?: { id: string; label: string }[] | undefined;
  /** Открытая вкладка связи. Пусто — открыта сама карточка. */
  tab?: string | undefined;
  onTab?: ((id: string) => void) | undefined;
  /** Содержимое открытой вкладки связи. */
  tabContent?: ReactNode;
  onEdit?: ((guid: string, slug: string, value: unknown) => void) | undefined;
  onSettings?: ((field: Field, anchor: DOMRect) => void) | undefined;
  /**
   * Поле `moved` переставлено к `target` — до него или после.
   *
   * Сообщается сама перестановка, а не новый список: порядок карточки
   * хранится отдельно от колонок таблицы и содержит в том числе поля,
   * которых в drawer нет (скрытые из view). Собрать его из показанных
   * полей нельзя — скрытые бы потерялись.
   *
   * Не задан — поля не перетаскиваются.
   */
  onReorder?: ((moved: string, target: string, after: boolean) => void) | undefined;
  /**
   * Заголовком карточки назначено другое поле. `variants` — карта
   * «язык → слаг» у мультиязычного поля: заголовок нужен на каждом языке.
   *
   * Не задан — заголовок не меняется (нет прав на настройки).
   */
  onHeading?: ((slug: string, variants: Record<string, string> | null) => void) | undefined;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  /** Какое поле правится. Одно на drawer — как и в таблице. */
  const [active, setActive] = useState<{ slug: string; anchor: DOMRect } | null>(null);
  const { drawerMode, setDrawerMode, drawerWidth, setDrawerWidth } = useUi();
  const panel = useRef<HTMLElement>(null);
  const side = drawerMode === "side";

  /*
   * Язык ДАННЫХ, на котором показана карточка. Своё состояние, а не общий
   * язык проекта: переключатель — свойство карточки, и переключение
   * не должно менять подписи вариантов во всей таблице.
   *
   * Мультиязычное поле — это несколько КОЛОНОК со слагами `title_en`,
   * `title_cyr`; без сборки карточка показывает их тремя соседними
   * строками с одинаковой подписью.
   */
  const codes = useMemo(() => languages.map((item) => item.code), [languages]);
  const [dataLanguage, setDataLanguage] = useState(language);
  const multilingual = useMemo(() => hasMultilanguage(columns, codes), [columns, codes]);

  const fields = useMemo(
    () => (multilingual ? fieldsForLanguage(columns, codes, dataLanguage) : columns),
    [multilingual, columns, codes, dataLanguage],
  );

  /*
   * Escape закрывает drawer — но только когда поверх него ничего нет.
   * У открытого редактора свой Escape («отменить правку»), и одно
   * нажатие не должно делать оба действия сразу.
   */
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !active) onClose();
    };

    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [active, onClose]);

  const guid = typeof row?.guid === "string" ? row.guid : undefined;

  /*
   * Заголовок карточки — поле, НАЗНАЧЕННОЕ админом (layout_heading),
   * а не первая колонка view. Колонки таблицы переставляют часто,
   * и заголовок карточки уезжал бы вместе с ними.
   *
   * Заголовок не участвует в перетаскивании и в список полей не входит:
   * он уже показан сверху, и вторая его копия ниже читается как дубль.
   * Не назначен — заголовка нет: угадывать «первое текстовое» значит
   * молча выбрать за админа, и в карточке появится поле, которого он
   * там не ставил.
   */
  const title = heading ? fields.find((field) => field.slug === heading) : undefined;
  const rest = title ? fields.filter((field) => field !== title) : fields;
  // Связи ищутся по id на каждом поле-ссылке — держим индексом.
  const byId = new Map(relations.map((relation) => [relation.id, relation]));

  /**
   * Поля, разложенные по секциям раскладки. Секция без своих полей
   * не рисуется вовсе, а всё, чего в секциях нет, идёт последней
   * безымянной группой: новое поле бэкенд дописывает в раскладку не
   * мгновенно, и до тех пор оно не должно пропадать из карточки.
   */
  const groups = useMemo(() => groupBySection(rest, sections), [rest, sections]);

  const open = (field: Field, element: HTMLElement) => {
    // Флажок переключается на месте — как в таблице.
    if (row && guid && onEdit && editorKind(field) === "boolean") {
      onEdit(guid, field.slug, !row[field.slug]);
      return;
    }

    setActive({ slug: field.slug, anchor: element.getBoundingClientRect() });
  };

  const activeField = active ? fields.find((field) => field.slug === active.slug) : undefined;

  /*
   * Перетаскивание полей. Нативное HTML5, как в сайдбаре: переставить
   * соседей в одном списке — вся задача, а dnd-kit стоил бы 40 КБ.
   *
   * Состояния два: что тащат и над кем сейчас курсор. Второе нужно
   * не только для линии — по нему же считается бросок: событие drop
   * приходит от строки, но половину строки (верх или низ) знает только
   * dragover.
   */
  const [dragged, setDragged] = useState<string | null>(null);
  const [over, setOver] = useState<{ slug: string; after: boolean } | null>(null);

  const endDrag = () => {
    setDragged(null);
    setOver(null);
  };

  const drop = () => {
    if (dragged && over) onReorder?.(dragged, over.slug, over.after);
    endDrag();
  };

  return (
    <>
      {/* Затемнение — только у центрального положения: сбоку таблица
          остаётся рабочей, во весь экран её и не видно. */}
      {drawerMode === "center" && (
        <div
          className="fixed inset-0 z-40"
          style={{ background: "var(--color-overlay)" }}
          onClick={onClose}
        />
      )}

      <aside
        ref={panel}
        style={side ? { width: drawerWidth } : undefined}
        className={`fixed z-50 flex flex-col bg-surface ${MODE_CLASS[drawerMode]}`}
      >
        {side && (
          <ResizeHandle
            edge="left"
            target={panel}
            value={drawerWidth}
            min={DRAWER_MIN_WIDTH}
            max={DRAWER_MAX_WIDTH}
            label={t("drawer.resize")}
            onCommit={setDrawerWidth}
            /*
             * Широкой панели сайдбар только мешает: места под таблицу
             * не остаётся вовсе. Состояние берётся из стора, а не из
             * замыкания: замыкание создаётся один раз на нажатие и не
             * узнает, что сайдбар уже свёрнут — сворачивало бы его
             * на каждом движении мыши.
             */
            onDrag={(width) => {
              const ui = useUi.getState();
              if (width > SIDEBAR_YIELDS_AT && !ui.sidebarCollapsed) ui.toggleSidebar();
            }}
          />
        )}

        <header className="flex h-header shrink-0 items-center gap-1 px-2">
          <IconButton
            icon={side ? IconChevronsRight : IconX}
            label={t("action.close")}
            onClick={onClose}
          />

          <Popover
            trigger={({ toggle }) => (
              <IconButton
                icon={MODE_ICON[drawerMode]}
                label={t("drawer.mode")}
                onClick={toggle}
              />
            )}
          >
            {(close) => (
              <div className="w-52">
                {MODES.map((mode) => (
                  <PopoverItem
                    key={mode}
                    icon={<Icon as={MODE_ICON[mode]} size={16} className="text-fg-muted" />}
                    onClick={() => {
                      setDrawerMode(mode);
                      close();
                    }}
                  >
                    <span className="flex items-center gap-2">
                      {t(MODE_LABEL[mode])}
                      {drawerMode === mode && <Icon as={IconCheck} size={14} />}
                    </span>
                  </PopoverItem>
                ))}
              </div>
            )}
          </Popover>

          {/* Переключатель языка ДАННЫХ — только когда есть что переключать.
              У таблицы без мультиязычных полей он не менял бы ничего. */}
          {multilingual && (
            <div className="ml-auto flex shrink-0 items-center gap-0.5">
              {languages.map((item) => (
                <button
                  key={item.code}
                  type="button"
                  onClick={() => setDataLanguage(item.code)}
                  title={item.nativeName}
                  className={`h-6 rounded-md px-1.5 text-xs transition-colors ${
                    dataLanguage === item.code
                      ? "bg-accent-subtle text-accent-text"
                      : "text-fg-subtle hover:bg-surface-hover hover:text-fg"
                  }`}
                >
                  {item.code}
                </button>
              ))}
            </div>
          )}
        </header>

        {/*
         * Вкладки связей. Первая — сама карточка: у неё нет своего view,
         * и в старой админке она тоже стоит в этом ряду. Ряд появляется
         * только когда связи есть — одна вкладка «Запись» ничего
         * не переключает.
         */}
        {row && tabs && tabs.length > 0 && (
          <div className="flex h-9 shrink-0 items-center gap-1 overflow-x-auto border-b border-border px-3">
            <Tab label={t("drawer.record")} active={!tab} onClick={() => onTab?.("")} />
            {tabs.map((item) => (
              <Tab
                key={item.id}
                label={item.label}
                active={tab === item.id}
                onClick={() => onTab?.(item.id)}
              />
            ))}
          </div>
        )}

        {!row ? (
          <p className="p-6 text-sm text-fg-muted">{t("drawer.notFound")}</p>
        ) : tab ? (
          tabContent
        ) : (
          <div className="min-h-0 flex-1 overflow-y-auto px-6 pb-10">
            <Heading
              field={title}
              candidates={rest}
              language={language}
              dataLanguage={dataLanguage}
              codes={codes}
              onPick={onHeading}
              onOpen={open}
            >
              {title && (
                <Cell
                  field={title}
                  row={row}
                  tableSlug={tableSlug}
                  relations={byId}
                  locale={locale}
                  language={language}
                  wrap
                />
              )}
            </Heading>

            {groups.map((group) => (
              <section key={group.label || "—"}>
                {/* Заголовок секции — только у именованной: у карточки почти
                    всегда одна безымянная секция, и пустая полоска над ней
                    читается как сломанная вёрстка. */}
                {group.label && (
                  <h3 className="mt-4 mb-1 px-1 text-2xs font-medium tracking-wide text-fg-subtle uppercase">
                    {group.label}
                  </h3>
                )}

                {group.fields.map((field) => (
                  <div
                    key={field.id}
                    /*
                     * Цель броска — строка целиком, а тащат за подпись:
                     * ронять поле в узкую полоску подписи неудобно, а
                     * начинать перетаскивание с ячейки значения нельзя —
                     * из неё выделяют текст.
                     */
                    onDragOver={(event) => {
                      if (!dragged || dragged === field.slug) return;

                      // Без preventDefault браузер запрещает бросок, и drop
                      // не придёт вовсе.
                      event.preventDefault();
                      const box = event.currentTarget.getBoundingClientRect();
                      const after = event.clientY - box.top > box.height / 2;

                      setOver((current) =>
                        current?.slug === field.slug && current.after === after
                          ? current
                          : { slug: field.slug, after },
                      );
                    }}
                    onDrop={(event) => {
                      event.preventDefault();
                      drop();
                    }}
                    className={`relative flex items-start gap-2 py-0.5 transition-opacity ${
                      dragged === field.slug ? "opacity-40" : ""
                    }`}
                  >
                    {over?.slug === field.slug && (
                      <span
                        className={`pointer-events-none absolute inset-x-0 h-0.5 rounded-full bg-accent ${
                          over.after ? "bottom-0" : "top-0"
                        }`}
                      />
                    )}

                    {/* Код языка из подписи убирается: он уже выбран
                        переключателем в шапке, и «Название (en)» под
                        кнопкой «en» — это то же слово дважды. */}
                    <FieldLabel
                      label={stripLanguage(
                        localized(field.labels, language, field.label),
                        multilingual ? dataLanguage : "",
                      )}
                      field={field}
                      draggable={Boolean(onReorder)}
                      onDragStart={(event) => {
                        // Без данных в dataTransfer Firefox не начинает
                        // перетаскивание вовсе.
                        event.dataTransfer.setData("text/plain", field.slug);
                        event.dataTransfer.effectAllowed = "move";
                        setDragged(field.slug);
                      }}
                      onDragEnd={endDrag}
                    />

                    {/*
                     * У BUTTON ячейка сама по себе кнопка: обернуть её
                     * во вторую — невалидный HTML (кнопка в кнопке),
                     * и браузер по вложенному щелчку ведёт себя как
                     * захочет. Открывать здесь всё равно нечего: BUTTON
                     * — это действие, а не значение.
                     */}
                    {cellKind(field.type) === "button" ? (
                      <div className="flex min-h-8 min-w-0 flex-1 items-center px-1.5 py-1 text-sm">
                        <Cell
                          field={field}
                          row={row}
                          tableSlug={tableSlug}
                          relations={byId}
                          locale={locale}
                          language={language}
                          wrap
                        />
                      </div>
                    ) : (
                      <button
                        type="button"
                        onClick={(event) => open(field, event.currentTarget)}
                        className="flex min-h-8 min-w-0 flex-1 items-center rounded-md px-1.5 py-1 text-left text-sm transition-colors hover:bg-surface-hover"
                      >
                        <Cell
                          field={field}
                          row={row}
                          tableSlug={tableSlug}
                          relations={byId}
                          locale={locale}
                          language={language}
                          wrap
                        />
                      </button>
                    )}
                  </div>
                ))}
              </section>
            ))}
          </div>
        )}

        {active && activeField && row && (
          <ActiveCell
            key={active.slug}
            field={activeField}
            row={row}
            guid={onEdit ? guid : undefined}
            tableSlug={tableSlug}
            anchor={active.anchor}
            relations={byId}
            locale={locale}
            language={language}
            heading={active.slug === title?.slug}
            onSettings={onSettings}
            onEdit={(value) => {
              if (guid) onEdit?.(guid, active.slug, value);
            }}
            onClose={() => setActive(null)}
          />
        )}
      </aside>
    </>
  );
}

const MODES: DrawerMode[] = ["side", "center", "full"];

const MODE_LABEL = {
  side: "drawer.sidePeek",
  center: "drawer.centerPeek",
  full: "drawer.fullPage",
} as const;

const MODE_ICON = {
  side: IconLayoutSidebarRightExpand,
  center: IconSquareToggleHorizontal,
  full: IconSquare,
} as const;

/**
 * Сбоку — панель у правого края, тянется за левый край. По центру —
 * карточка над таблицей. Во весь экран — без скруглений и границ:
 * это уже не панель поверх экрана, а сам экран.
 */
const MODE_CLASS = {
  side: "inset-y-0 right-0 border-l border-border shadow-modal",
  center:
    "inset-y-8 left-1/2 w-[min(1100px,calc(100%-4rem))] -translate-x-1/2 rounded-xl border border-border shadow-modal",
  full: "inset-0",
} as const;

/** Шире этого drawer забирает экран себе — сайдбар уступает место. */
const SIDEBAR_YIELDS_AT = 800;

/**
 * Заголовок карточки — значение поля, назначенного заголовком.
 *
 * Поле выбирает админ, и хранится выбор в раскладке (`layout_heading`),
 * а не берётся первой колонкой view: колонки таблицы переставляют часто,
 * а заголовок карточки при этом меняться не должен.
 *
 * Заголовок не назначен — вместо него кнопка «выбрать поле». Подставлять
 * первое текстовое нельзя: это выбор за админа, и в карточке появилось бы
 * поле, которого он туда не ставил.
 *
 * Кандидаты — только текстовые: заголовком карточки бывает название,
 * а не дата и не флажок. Тот же список, что и в старой админке.
 */
function Heading({
  field,
  candidates,
  language,
  dataLanguage,
  codes,
  children,
  onPick,
  onOpen,
}: {
  field: Field | undefined;
  candidates: Field[];
  language: string;
  dataLanguage: string;
  codes: string[];
  children: ReactNode;
  onPick: ((slug: string, variants: Record<string, string> | null) => void) | undefined;
  onOpen: (field: Field, element: HTMLElement) => void;
}) {
  const { t } = useTranslation();

  const pick = (chosen: Field) => {
    /*
     * У мультиязычного поля заголовок пишется картой по всем языкам
     * сразу: человек выбирает поле один раз, а карточка открывается
     * на любом языке — иначе переключение языка обнуляло бы заголовок.
     */
    const base = baseSlug(chosen, codes);
    const variants = base
      ? Object.fromEntries(codes.map((code) => [code, `${base}_${code}`]))
      : null;

    onPick?.(chosen.slug, variants);
  };

  const chooser = onPick && (
    <Popover
      align="end"
      trigger={({ toggle }) => (
        <button
          type="button"
          onClick={toggle}
          aria-label={t("drawer.heading")}
          title={t("drawer.heading")}
          className="grid size-7 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-hover hover:text-fg"
        >
          <Icon as={IconHeading} size={16} />
        </button>
      )}
    >
      {(close) => {
        const options = candidates.filter((item) => HEADING_TYPES.has(item.type));

        return (
          <div className="max-h-72 w-56 overflow-y-auto">
            {options.map((item) => (
              <PopoverItem
                key={item.id}
                active={item.slug === field?.slug}
                icon={<Icon as={fieldIcon(item.type)} size={16} className="shrink-0" />}
                onClick={() => {
                  pick(item);
                  close();
                }}
              >
                {stripLanguage(
                  localized(item.labels, language, item.label),
                  codes.length ? dataLanguage : "",
                )}
              </PopoverItem>
            ))}

            {!options.length && (
              <p className="px-2 py-2 text-xs text-fg-subtle">{t("table.noFields")}</p>
            )}
          </div>
        );
      }}
    </Popover>
  );

  return (
    <div className="mt-2 mb-4 flex items-start gap-1">
      {field ? (
        <button
          type="button"
          onClick={(event) => onOpen(field, event.currentTarget)}
          /* Отступы те же, что у редактора (карточка p-1.5 + поле
             px-0.5): текст не сдвигается в момент открытия правки. */
          className="flex min-w-0 flex-1 rounded-md px-2 py-1.5 text-left text-2xl leading-8 font-semibold transition-colors hover:bg-surface-hover"
        >
          {children}
        </button>
      ) : (
        <span className="flex-1 px-2 py-1.5 text-2xl leading-8 font-semibold text-fg-subtle">
          {t("drawer.noHeading")}
        </span>
      )}

      {chooser}
    </div>
  );
}

function Tab({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={`h-7 shrink-0 rounded-md px-2 text-sm transition-colors ${
        active
          ? "bg-accent-subtle text-accent-text"
          : "text-fg-muted hover:bg-surface-hover hover:text-fg"
      }`}
    >
      {label}
    </button>
  );
}

/**
 * Типы, которыми бывает заголовок карточки. Тот же набор, что и в старой
 * админке: заголовок — это название строки, а не её дата или галочка.
 */
const HEADING_TYPES = new Set(["SINGLE_LINE", "MULTI_LINE", "TEXT", "INCREMENT_ID"]);

/**
 * Поля, разложенные по секциям раскладки.
 *
 * Секций нет — одна безымянная группа со всеми полями. Всё, чего в
 * секциях не оказалось, идёт последней безымянной группой: новое поле
 * бэкенд дописывает в раскладку не мгновенно, и до тех пор оно не должно
 * пропадать из карточки.
 */
function groupBySection(
  fields: Field[],
  sections: { label: string; slugs: string[] }[],
): { label: string; fields: Field[] }[] {
  if (!sections.length) return [{ label: "", fields }];

  const taken = new Set<string>();
  const groups = sections.map((section) => {
    const inSection = fields.filter((field) => {
      if (!section.slugs.includes(field.slug) || taken.has(field.slug)) return false;
      taken.add(field.slug);
      return true;
    });

    return { label: section.label, fields: inSection };
  });

  const rest = fields.filter((field) => !taken.has(field.slug));
  if (rest.length) groups.push({ label: "", fields: rest });

  return groups.filter((group) => group.fields.length);
}

/**
 * Подпись поля слева от значения. Она же ручка перетаскивания.
 *
 * Под курсором иконка типа сменяется ручкой, а полное имя показывается
 * подсказкой: колонка подписей шириной 160px, и длинное имя в ней
 * обрезано — прочитать его иначе нечем.
 *
 * Тащат именно за подпись, а не за строку целиком: в ячейке значения
 * выделяют текст, и `draggable` на ней отнимал бы выделение.
 *
 * Подсказка — на CSS, без состояния и таймеров: задержка задаётся
 * `delay` у перехода прозрачности. Показывается сверху, а не сбоку:
 * слева от подписи всего 24px отступа, и всё, что уедет за них, обрежет
 * прокручиваемая область.
 */
function FieldLabel({
  label,
  field,
  draggable,
  onDragStart,
  onDragEnd,
}: {
  label: string;
  field: Field;
  draggable: boolean;
  onDragStart: (event: DragEvent<HTMLElement>) => void;
  onDragEnd: () => void;
}) {
  return (
    <span
      draggable={draggable}
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      className={`group/label relative flex h-8 w-40 shrink-0 items-center gap-1.5 rounded-md px-1 text-sm text-fg-muted transition-colors hover:bg-surface-hover ${
        draggable ? "cursor-grab active:cursor-grabbing" : ""
      }`}
    >
      <span className="relative grid size-3.5 place-items-center">
        <Icon
          as={fieldIcon(field.type)}
          size={14}
          className="transition-opacity group-hover/label:opacity-0"
        />
        <Icon
          as={IconGripVertical}
          size={14}
          className="absolute opacity-0 transition-opacity group-hover/label:opacity-100"
        />
      </span>

      <span className="truncate">{label}</span>

      <span className="pointer-events-none absolute bottom-full left-0 z-10 mb-1 max-w-72 truncate rounded-md border border-border bg-surface px-1.5 py-0.5 text-xs text-fg opacity-0 shadow-popover transition-opacity delay-0 group-hover/label:opacity-100 group-hover/label:delay-500">
        {label}
      </span>
    </span>
  );
}

function IconButton({
  icon,
  label,
  onClick,
}: {
  icon: typeof IconChevronsRight;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      title={label}
      className="grid size-7 place-items-center rounded-md text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
    >
      <Icon as={icon} size={16} />
    </button>
  );
}
