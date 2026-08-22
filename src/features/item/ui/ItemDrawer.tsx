import {
  Fragment,
  useEffect,
  useMemo,
  useRef,
  useState,
  type DragEvent,
  type ReactNode,
} from "react";
import {
  IconCheck,
  IconChevronLeft,
  IconChevronRight,
  IconChevronsRight,
  IconFileDescription,
  IconFileTypePdf,
  IconGripVertical,
  IconHeading,
  IconLayoutList,
  IconLayoutSidebarRightExpand,
  IconPlus,
  IconSquare,
  IconSquareToggleHorizontal,
  IconTable,
  IconX,
  type Icon as TablerIcon,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import {
  collapseLanguages,
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
import { CommitInput } from "@/shared/ui/commit-input";
import { Icon } from "@/shared/ui/icon";
import { Tooltip } from "@/shared/ui/tooltip";
import { LanguageTabs } from "@/shared/ui/language-tabs";
import { Popover, PopoverItem } from "@/shared/ui/popover";
import { ResizeHandle } from "@/shared/ui/resize-handle";
import { Tabs } from "@/shared/ui/tabs";
import { cellKind, editorKind } from "../model/cell-kind";
import { itemTitle } from "../model/layout";
import type { Item } from "../model/types";
import { Cell } from "./Cell";
import { ActiveCell } from "./CellEditor";
import { fieldIcon } from "./field-icon";

/**
 * Открытые карточки в порядке появления. Верхняя — последняя.
 * Модуль, а не контекст: знать друг о друге карточкам больше незачем,
 * а провайдер ради одного массива — это провайдер ради одного массива.
 */
const DRAWER_STACK: object[] = [];

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
 * Строки нет в загруженной странице — вызывающий запрашивает её по guid
 * (см. useItem): по пересланной ссылке карточка открывается у того, у кого
 * свой отбор и своя страница.
 */
export function ItemDrawer({
  tableSlug,
  columns,
  row,
  loading = false,
  error = null,
  relations,
  locale,
  language,
  languages,
  sections,
  heading,
  trail,
  tabs,
  tab,
  onTab,
  tabContent,
  onLanguage,
  onAddTab,
  addableRelations,
  tabTypes,
  onPdf,
  actions,
  titlePlaceholder,
  footer,
  onEdit,
  onLink,
  onSettings,
  onReorder,
  onAddSection,
  onRenameSection,
  onRemoveSection,
  onHeading,
  onClose,
}: {
  tableSlug: string;
  /** Поля записи в порядке карточки — своём, не в порядке колонок таблицы. */
  columns: Field[];
  row: Item | undefined;
  /** Строка ещё едет: карточка открыта по ссылке, а страницы под ней нет. */
  loading?: boolean;
  /** Причина отказа словами. Пусто — отказа не было. */
  error?: string | null;
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
   * Откуда открыта карточка: предки — таблица, запись, вкладка связи.
   * Сама карточка дописывается последней крошкой и не нажимается.
   *
   * Крошки — готовые подписи и обработчики, а не идентификаторы: путь
   * знает тот, кто карточку открыл, и запрашивать его заново неоткуда
   * и незачем — все имена уже показаны на экране.
   *
   * Пусто — крошек нет вовсе: одна своя крошка повторяла бы заголовок,
   * который и так стоит ниже.
   */
  trail?: { label: string; onClick: () => void }[] | undefined;
  /**
   * Вкладки связей: связанные строки рядом с карточкой. Их содержимое
   * рисует вызывающий — это чужая таблица со своими колонками, и знать
   * о ней карточка не обязана. Пусто — вкладок нет, показана только карточка.
   */
  tabs?:
    | {
        id: string;
        label: string;
        relationId?: string;
        /** Куда смотрит связь: от этого зависит значок вкладки. */
        direction?: "incoming" | "outgoing";
      }[]
    | undefined;
  /**
   * Связи, из которых заводится вкладка. Отбирает их вызывающий: какая
   * связь показывается вкладкой, знает features/view, а карточка только
   * рисует список.
   */
  addableRelations?: Relation[] | undefined;
  /** Открытая вкладка связи. Пусто — открыта сама карточка. */
  tab?: string | undefined;
  onTab?: ((id: string) => void) | undefined;
  /** Содержимое открытой вкладки связи. */
  tabContent?: ReactNode;
  /** Сменить язык ДАННЫХ. Не задан — переключателя нет. */
  onLanguage?: ((code: string) => void) | undefined;
  /**
   * Завести вкладку по связи. Не задан — «+» в полосе вкладок нет:
   * раскладку правит тот же, кто правит настройки view.
   */
  onAddTab?: ((relationId: string, label: string, type: string) => void) | undefined;
  /**
   * Типы, которыми бывает вкладка связи: подпись и значок на каждый.
   * Набор знает features/view — карточке о типах view знать незачем,
   * она только рисует список. Пусто или один — шага выбора нет вовсе.
   */
  tabTypes?: { type: string; label: string; icon: TablerIcon }[] | undefined;
  /**
   * Открыть PDF записи. Адрес задаёт админ в настройках view
   * (attributes.pdf_url); не задан — кнопки нет.
   */
  onPdf?: (() => void) | undefined;
  /**
   * Кнопки, которые карточка не рисует сама: действия таблицы над
   * открытой строкой. Их набор знает вызывающая страница — карточке
   * о функциях проекта знать незачем.
   */
  actions?: ReactNode;
  /**
   * Чем подписать карточку, у которой заголовка нет: у новой записи
   * это «Новая запись», а не «Без заголовка» — заголовок ей ещё
   * неоткуда взять.
   */
  titlePlaceholder?: string;
  /**
   * Полоса внизу карточки: у новой записи это «Создать». У открытой
   * строки её нет вовсе — правка ячейки уезжает сразу, и кнопка
   * «сохранить» обещала бы, что без неё ничего не сохранилось.
   */
  footer?: ReactNode;
  onEdit?: ((guid: string, slug: string, value: unknown) => void) | undefined;
  /**
   * Выбранная строка связи — вместо запроса «связать». Нужно черновику
   * новой записи: строки в базе ещё нет, и PUT ушёл бы по несуществующему
   * guid (см. ActiveCell.onLink). Не задан — связь уезжает запросом.
   */
  onLink?: ((slug: string, item: Item | null) => void) | undefined;
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
   * Правка секций карточки. Не заданы — секции только показываются:
   * раскладку правит тот же, кому позволено её двигать.
   */
  onAddSection?: ((label: string) => void) | undefined;
  onRenameSection?: ((index: number, label: string) => void) | undefined;
  onRemoveSection?: ((index: number) => void) | undefined;
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
  const { drawerMode, setDrawerMode, drawerWidth, setDrawerWidth, sidebarCollapsed } = useUi();
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
  const multilingual = useMemo(() => hasMultilanguage(columns, codes), [columns, codes]);

  /*
   * Языковые колонки сводит вызывающая страница — тем же языком, что
   * и таблица. Здесь остаётся сборка на случай, когда карточку открыли
   * с полным набором полей: она идемпотентна.
   */
  const fields = useMemo(
    () => (multilingual ? collapseLanguages(columns, codes, language) : columns),
    [multilingual, columns, codes, language],
  );

  /*
   * Escape закрывает drawer — но только когда поверх него ничего нет.
   * У открытого редактора свой Escape («отменить правку»), и одно
   * нажатие не должно делать оба действия сразу.
   *
   * Карточек на экране бывает две: связанная строка раскрывается
   * поверх вкладки связи. Закрывается верхняя — иначе одно нажатие
   * уносило бы обе, а человек хотел вернуться к списку. Отсюда стопка:
   * слушателей два, но действует тот, кто в ней последний.
   */
  useEffect(() => {
    const self = {};
    DRAWER_STACK.push(self);

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || active) return;
      if (DRAWER_STACK[DRAWER_STACK.length - 1] !== self) return;

      onClose();
    };

    document.addEventListener("keydown", onKeyDown);

    return () => {
      document.removeEventListener("keydown", onKeyDown);
      const at = DRAWER_STACK.indexOf(self);
      if (at !== -1) DRAWER_STACK.splice(at, 1);
    };
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

  /*
   * Крошки: путь до карточки и она сама последней. Своя крошка
   * нажимается только при открытой вкладке связи — тогда она возвращает
   * к самой записи. Без вкладки нажимать нечего: мы уже здесь.
   */
  /*
   * Вкладки карточки. Первая — сама запись: у неё нет своего view,
   * и в старой админке она тоже стоит в этом ряду.
   */
  const tabItems = useMemo(
    () => [
      { id: "", label: t("drawer.record"), icon: IconLayoutList },
      ...(tabs ?? []).map((item) => ({
        id: item.id,
        label: item.label,
        /* Вкладка на одну строку — не таблица: у обратной связи
           в нашей колонке лежит ровно один чужой guid. */
        icon: item.direction === "outgoing" ? IconFileDescription : IconTable,
      })),
    ],
    [tabs, t],
  );

  const crumbs = [
    ...(trail ?? []),
    {
      label: itemTitle(row, title?.slug ?? "") || titlePlaceholder || t("drawer.noHeading"),
      ...(tab && onTab ? { onClick: () => onTab("") } : {}),
    },
  ];
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
        /*
         * Свёрнутый сайдбар убирает отступ и скругление у контента
         * (`_authed.tsx`: отделять карточку слева не от чего) — панель
         * следует за ним и тоже прижимается к краю. Иначе она осталась
         * бы единственной плавающей поверхностью на экране, где всё
         * остальное лежит встык.
         */
        className={`fixed z-50 flex flex-col bg-surface ${
          side && sidebarCollapsed ? SIDE_FLUSH : MODE_CLASS[drawerMode]
        }`}
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

        {/* Отступы и граница — как у шапки страницы: это тот же ряд,
            только в панели. */}
        <header className="flex h-header shrink-0 items-center gap-1 border-b border-border px-4">
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

          {/*
           * Путь до карточки. Открытая поверх вкладки связанная строка
           * прячет собой и таблицу, и запись, из которой её раскрыли,
           * — крошки единственное, что показывает, где мы находимся.
           *
           * Одна крошка не рисуется: она повторяла бы заголовок ниже.
           */}
          {crumbs.length > 1 && (
            <nav
              aria-label={t("drawer.trail")}
              className="flex min-w-0 flex-1 items-center gap-0.5 text-xs text-fg-muted"
            >
              {crumbs.map((crumb, index) => (
                <Fragment key={index}>
                  {index > 0 && (
                    <Icon as={IconChevronRight} size={12} className="shrink-0 text-fg-subtle" />
                  )}

                  {crumb.onClick ? (
                    <button
                      type="button"
                      onClick={crumb.onClick}
                      className="min-w-0 truncate rounded px-1 py-0.5 transition-colors hover:bg-surface-hover hover:text-fg"
                    >
                      {crumb.label}
                    </button>
                  ) : (
                    <span className="min-w-0 truncate px-1 py-0.5 text-fg">{crumb.label}</span>
                  )}
                </Fragment>
              ))}
            </nav>
          )}

          {/*
           * Правая группа шапки: сначала показ, потом действия.
           *
           * Переключатель языка — про то, КАК показана запись, и стоит
           * он первым, рядом с крошками: те тоже про показ. PDF и действия
           * — про саму запись, и они у самого края, где их и ищут.
           *
           * Группа одна на все три: без неё язык прижимался к правому
           * краю в одиночку, а действия оставались посреди шапки —
           * и по три кнопки в разных её местах читались как три разных
           * ряда.
           */}
          <div className="ml-auto flex shrink-0 items-center gap-1">
            {multilingual && onLanguage && (
              <LanguageTabs languages={languages} value={language} onChange={onLanguage} />
            )}

            {/* PDF записи: адрес задан в настройках view, и печатная форма
                нужна прямо здесь — с открытой карточкой, а не после
                возврата в таблицу. */}
            {onPdf && (
              <IconButton icon={IconFileTypePdf} label={t("drawer.openPdf")} onClick={onPdf} />
            )}

            {actions}
          </div>
        </header>

        {/*
         * Вкладки связей. Первая — сама карточка: у неё нет своего view,
         * и в старой админке она тоже стоит в этом ряду. Ряд появляется
         * только когда связи есть — одна вкладка «Запись» ничего
         * не переключает.
         */}
        {row && ((tabs && tabs.length > 0) || onAddTab) && (
          <div className="flex h-11 shrink-0 items-center gap-2 border-b border-border px-3">
            {/* Прокрутку и плашку активной вкладки держит сама полоса
                (shared/ui/tabs). «+» стоит рядом с ней, а не внутри:
                всплывашка, открытая из прокручиваемого контейнера,
                обрезается его краями. */}
            <Tabs
              tabs={tabItems}
              activeId={tab ?? ""}
              onSelect={(id) => onTab?.(id)}
            />

            {/* Новая вкладка — это связь, которую ещё не показали:
                бэкенд заводит вкладки сам, но только тем связям,
                что существовали на момент создания раскладки. */}
            {onAddTab && (
              <AddTabButton
                relations={addableRelations ?? []}
                shown={new Set((tabs ?? []).map((item) => item.relationId).filter(Boolean))}
                language={language}
                types={tabTypes ?? []}
                onAdd={onAddTab}
              />
            )}
          </div>
        )}

        {!row ? (
          /*
           * Строки нет — и это три разных случая, а не один. Пока она
           * едет, «записи не существует» — неправда; отказ сервера тем
           * более: он чинится, а «не найдено» нет.
           */
          <p className="p-6 text-sm text-fg-muted">
            {loading ? t("common.loading") : (error ?? t("drawer.notFound"))}
          </p>
        ) : tab ? (
          tabContent
        ) : (
          <div className="min-h-0 flex-1 overflow-y-auto px-6 pb-10">
            <Heading
              field={title}
              candidates={rest}
              language={language}
              codes={codes}
              placeholder={titlePlaceholder ?? t("drawer.noHeading")}
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

            {groups.map((group, index) => (
              <section key={group.label || `—${index}`}>
                {/* Заголовок секции — только у именованной: у карточки почти
                    всегда одна безымянная секция, и пустая полоска над ней
                    читается как сломанная вёрстка.
 
                    С правом на раскладку заголовок правится на месте:
                    отдельного экрана настроек у секции нет, а имя — это
                    всё, что у неё есть. */}
                {(group.label || onRenameSection) && (
                  <div className="group/section mt-4 mb-1 flex items-center gap-1 px-1">
                    {onRenameSection && group.index >= 0 ? (
                      <CommitInput
                        value={group.label}
                        label={t("drawer.sectionName")}
                        placeholder={t("drawer.sectionUnnamed")}
                        allowEmpty
                        onCommit={(label) => onRenameSection(group.index, label)}
                        className="h-6 border-transparent bg-transparent px-1 text-2xs font-medium tracking-wide uppercase"
                      />
                    ) : (
                      <h3 className="text-2xs font-medium tracking-wide text-fg-subtle uppercase">
                        {group.label}
                      </h3>
                    )}

                    {onRemoveSection && group.index > 0 && (
                      <button
                        type="button"
                        onClick={() => onRemoveSection(group.index)}
                        aria-label={t("drawer.sectionRemove")}
                        title={t("drawer.sectionRemove")}
                        className="grid size-5 shrink-0 place-items-center rounded text-fg-subtle opacity-0 transition hover:bg-surface-hover hover:text-danger group-hover/section:opacity-100"
                      >
                        <Icon as={IconX} size={12} />
                      </button>
                    )}
                  </div>
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
                    /* group/row — для кнопок по наведению внутри ячейки:
                       «скопировать» у текста, переходы у ссылки и карты. */
                    className={`group/row relative flex items-start gap-2 py-0.5 transition-opacity ${
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
                        multilingual ? language : "",
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
                        {/*
                         * Длинный текст свёрнут в одну строку с «…»:
                         * абзац на пять строк растягивает карточку, и
                         * соседние поля уезжают за экран. Целиком его
                         * показывает раскрытый редактор — тот же клик.
                         */}
                        <Cell
                          field={field}
                          row={row}
                          tableSlug={tableSlug}
                          relations={byId}
                          locale={locale}
                          language={language}
                          wrap={cellKind(field.type) !== "longtext"}
                        />
                      </button>
                    )}
                  </div>
                ))}
              </section>
            ))}

            {/* Новая секция заводится пустой и тут же видна: в неё
                переносят поля мышью. */}
            {onAddSection && (
              <button
                type="button"
                onClick={() => onAddSection("")}
                className="mt-3 flex items-center gap-1.5 px-1 text-2xs text-fg-subtle transition-colors hover:text-fg"
              >
                <Icon as={IconPlus} size={12} />
                {t("drawer.sectionAdd")}
              </button>
            )}
          </div>
        )}

        {/* Полоса действий внизу: у новой записи — «Создать». */}
        {footer && (
          <div className="flex shrink-0 items-center justify-end gap-2 border-t border-border px-4 py-3">
            {footer}
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
            onLink={onLink ? (item) => onLink(active.slug, item) : undefined}
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
 *
 * У панели сбоку тот же отступ и тот же радиус, что у карточки контента
 * (`_authed.tsx`: `m-2 rounded-xl`): приклеенная к краю окна, она была
 * единственной поверхностью приложения со своими правилами — шапка
 * страницы начиналась на 8px ниже шапки панели, а скруглённый угол
 * контента уезжал под неё. Радиус 12px — тот, который DESIGN.md
 * и назначает drawer'у.
 */
const MODE_CLASS = {
  side: "inset-y-2 right-2 rounded-xl border border-border shadow-modal",
  center:
    "inset-y-8 left-1/2 w-[min(1100px,calc(100%-4rem))] -translate-x-1/2 rounded-xl border border-border shadow-modal",
  full: "inset-0",
} as const;

/** Панель сбоку без сайдбара: встык к краю окна, как и сам контент. */
const SIDE_FLUSH = "inset-y-0 right-0 border-l border-border shadow-modal";

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
  codes,
  placeholder,
  children,
  onPick,
  onOpen,
}: {
  field: Field | undefined;
  candidates: Field[];
  /** Язык ДАННЫХ: и подписи полей, и выбранный языковой вариант. */
  language: string;
  codes: string[];
  /** Что показать вместо значения. У новой записи это её имя. */
  placeholder: string;
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
                  codes.length ? language : "",
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
          {placeholder}
        </span>
      )}

      {chooser}
    </div>
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
): { label: string; index: number; fields: Field[] }[] {
  if (!sections.length) return [{ label: "", index: -1, fields }];

  const taken = new Set<string>();
  const groups = sections.map((section, index) => {
    const inSection = fields.filter((field) => {
      if (!section.slugs.includes(field.slug) || taken.has(field.slug)) return false;
      taken.add(field.slug);
      return true;
    });

    return { label: section.label, index, fields: inSection };
  });

  /* Всё, чего в секциях не оказалось, — последней безымянной группой.
     Номера у неё нет: править её как секцию нельзя, её в раскладке нет. */
  const rest = fields.filter((field) => !taken.has(field.slug));
  if (rest.length) groups.push({ label: "", index: -1, fields: rest });

  /*
   * Пустая секция остаётся видимой, когда раскладку правят: в неё
   * переносят поля мышью, а невидимая цель — это цель, которой нет.
   */
  return groups.filter((group) => group.fields.length || group.index >= 0);
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

      {/* Подсказка — слаг, а не подпись: подпись стоит рядом, а слаг
          это имя поля в API, фильтрах и формулах, и больше его нигде
          не видно. */}
      <Tooltip label={field.slug}>
        <span className="truncate">{label}</span>
      </Tooltip>
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

/**
 * «+» в полосе вкладок: показать связь этой таблицы.
 *
 * Два шага: связь, потом тип. Связь обязательна — как и в старой админке,
 * где «+ View» в карточке без неё не создаёт ничего
 * (useViewCreatePopupProps.jsx — `setError("table_slug")`). Тип раньше
 * не спрашивался вовсе, и вкладка молча получалась таблицей.
 *
 * Тип один — шага нет: список из одного пункта ничего не выбирает.
 *
 * Уже показанные связи из списка не убираются, только помечаются:
 * две вкладки на одну связь — это «все заказы» и «заказы за месяц»,
 * они отличаются колонками. Убирать их значило бы запретить второй
 * взгляд на те же строки.
 */
function AddTabButton({
  relations,
  shown,
  language,
  types,
  onAdd,
}: {
  relations: Relation[];
  shown: ReadonlySet<string | undefined>;
  language: string;
  /** Типы вкладки: подпись и значок. Пусто или один — шаг пропускается. */
  types: { type: string; label: string; icon: TablerIcon }[];
  onAdd: (relationId: string, label: string, type: string) => void;
}) {
  const { t } = useTranslation();
  /** Выбранная связь: пока её нет, показан первый шаг. */
  const [picked, setPicked] = useState<{ id: string; label: string } | null>(null);

  return (
    <Popover
      /* Влево от кнопки: она стоит у самого правого края карточки,
         и список, раскрытый вправо, уезжает за окно. */
      align="end"
      trigger={({ toggle }) => (
        <button
          type="button"
          onClick={() => {
            // Каждое открытие — с первого шага: выбор прошлого раза
            // к новой вкладке отношения не имеет.
            setPicked(null);
            toggle();
          }}
          aria-label={t("drawer.addTab")}
          title={t("drawer.addTab")}
          className="grid size-6 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-hover hover:text-fg"
        >
          <Icon as={IconPlus} size={14} />
        </button>
      )}
    >
      {(close) => {
        const create = (relationId: string, label: string, type: string) => {
          onAdd(relationId, label, type);
          setPicked(null);
          close();
        };

        if (picked) {
          return (
            <div className="max-h-72 w-56 overflow-y-auto">
              {/* Шаг назад, а не отдельное окно: выбранная связь написана
                  в заголовке — из него видно, к чему выбирается тип. */}
              <button
                type="button"
                onClick={() => setPicked(null)}
                className="flex h-8 w-full items-center gap-1 rounded-md px-1 text-left text-xs text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
              >
                <Icon as={IconChevronLeft} size={14} className="shrink-0" />
                <span className="truncate">{picked.label}</span>
              </button>

              {types.map((item) => (
                <PopoverItem
                  key={item.type}
                  icon={<Icon as={item.icon} size={16} className="shrink-0" />}
                  onClick={() => create(picked.id, picked.label, item.type)}
                >
                  {item.label}
                </PopoverItem>
              ))}
            </div>
          );
        }

        return (
          <div className="max-h-72 w-56 overflow-y-auto">
            <p className="px-2 py-1 text-2xs text-fg-subtle">{t("drawer.addTabHint")}</p>

            {relations.map((relation) => {
              /* Имя связи, а не таблицы: у двух связей на одну таблицу
                 иначе два одинаковых пункта. Та же цепочка, что и у самой
                 вкладки (model/layout, relationTabs). */
              const label =
                relation.title ||
                localized(relation.toLabels, language, relation.toLabel || relation.toSlug);

              return (
                <PopoverItem
                  key={relation.id}
                  active={shown.has(relation.id)}
                  trailing={
                    types.length > 1 ? (
                      <Icon as={IconChevronRight} size={14} className="text-fg-subtle" />
                    ) : undefined
                  }
                  onClick={() =>
                    types.length > 1
                      ? setPicked({ id: relation.id, label })
                      : create(relation.id, label, types[0]?.type ?? "TABLE")
                  }
                >
                  {label}
                </PopoverItem>
              );
            })}

            {!relations.length && (
              <p className="px-2 py-2 text-xs text-fg-subtle">{t("drawer.noRelations")}</p>
            )}
          </div>
        );
      }}
    </Popover>
  );
}
