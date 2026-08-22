import { useEffect, useMemo, useRef, useState, type DragEvent, type UIEvent } from "react";
import { IconPencil, IconPlus, IconX } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { localized, type Field, type Relation } from "@/features/table";
import { CHIP_SURFACE, Chip, hexToChipColor, type ChipColor } from "@/shared/ui/chip";
import { openPreview } from "@/shared/ui/file-preview";
import { Icon } from "@/shared/ui/icon";
import { Tooltip } from "@/shared/ui/tooltip";
import { boardColumns, boardOrderAt, groupValue, BOARD_ORDER } from "../model/board";
import { cellKind } from "../model/cell-kind";
import { isBlank } from "../model/cell-value";
import { relationSelection } from "../model/relation";
import type { Item } from "../model/types";
import { Cell } from "./Cell";
import { ActiveCell } from "./CellEditor";
import { fieldIcon } from "./field-icon";

/**
 * Доска: карточки, разложенные по колонкам значения одного поля.
 *
 * Данные — обычные строки таблицы (см. model/board о том, почему не
 * ручка доски), поэтому здесь только раскладка, перетаскивание и правка
 * карточки на месте.
 *
 * Прокрутка одна на всю доску — и вниз, и вбок: страницу сервер отдаёт
 * по всей доске сразу, а не по колонке. Шапки колонок при этом остаются
 * на виду: без них после первого экрана непонятно, в какой колонке
 * карточка.
 */

/** До конца прокрутки осталось меньше — заказываем следующую порцию. */
const END_GAP = 400;

/** Сколько ещё можно прокрутить вниз. */
const bottomGap = (area: HTMLElement) => area.scrollHeight - area.scrollTop - area.clientHeight;

/** React-ключ колонки «без значения»: пустая строка ключом не бывает. */
const NO_GROUP_KEY = "—";

export function Board({
  tableSlug,
  columns,
  rows,
  field,
  relations,
  locale,
  language,
  hasMore,
  onOpenRow,
  onMove,
  onEdit,
  onAddCard,
  onSettings,
  onEndReached,
}: {
  tableSlug: string;
  /** Поля карточки — колонки view, как и в таблице. */
  columns: Field[];
  rows: Item[];
  /** Поле группировки: его значения и есть колонки доски. */
  field: Field;
  relations: Relation[];
  locale: string;
  language: string;
  /** Загружены не все строки: счётчик в шапке колонки — с «+». */
  hasMore?: boolean | undefined;
  onOpenRow: (guid: string) => void;
  /**
   * Перенос карточки. Не задан — без права на правку доска остаётся
   * доской для чтения, а не рисует перетаскивание, которое ответит 403.
   */
  onMove?: ((guid: string, values: Record<string, unknown>) => void) | undefined;
  /** Правка поля прямо в карточке. Не задан — карточка только читается. */
  onEdit?: ((guid: string, slug: string, value: unknown) => void) | undefined;
  /** Новая запись сразу в колонку. Не задан — нет права на запись. */
  onAddCard?: ((columnId: string) => void) | undefined;
  /** Настройки поля из открытого редактора — как в таблице и карточке. */
  onSettings?: ((field: Field, anchor: DOMRect) => void) | undefined;
  onEndReached?: (() => void) | undefined;
}) {
  const { t } = useTranslation();
  /** Карточка в руке. Пусто — ничего не тащат. */
  const [dragging, setDragging] = useState<string | null>(null);
  /**
   * Та же карточка, но уже убранная из потока, — отдельным состоянием
   * и на кадр позже. Прятать её сразу нельзя: браузер снимает «призрак»
   * и НАЧИНАЕТ перетаскивание уже после обработчика dragstart, а React
   * успевает перерисовать раньше — элемент пропадает из потока, и
   * перетаскивание отменяется, не начавшись. Внешне это выглядит как
   * «мышь не берёт карточку».
   *
   * Почему отдельное состояние, а не задержка `dragging`: от кадра
   * зависит только вид. Само перетаскивание — что взяли и куда несут —
   * работает с первого события.
   */
  const [lifted, setLifted] = useState<string | null>(null);
  /** Куда её бросят: колонка и место в ней. */
  const [over, setOver] = useState<{ column: string; index: number } | null>(null);
  /** Высота карточки в руке: столько же занимает пустое место под неё. */
  const [height, setHeight] = useState(0);
  /**
   * Карточка, раскрытая на правку. Одна на доску: две открытые карточки
   * — это два места, куда уходит следующий щелчок.
   */
  const [editing, setEditing] = useState<string | null>(null);

  // Связи по id — ровно так их ждёт ячейка. Собираются здесь, как
  // и в таблице: наружу отдаётся тот же список, что приехал схемой.
  const byId = useMemo(
    () => new Map(relations.map((relation) => [relation.id, relation])),
    [relations],
  );

  const shown = useMemo(() => {
    /*
     * Варианты поля — те же и в том же порядке, что и вкладки таблицы
     * (features/view/api/tab-group): одна настройка, две раскладки.
     * У поля-связи вариантов нет — колонки соберутся из данных.
     */
    const tabs = [...field.options.values()].map((option) => ({
      id: option.value,
      label: localized(option.labels, language, option.label || option.value),
    }));

    /* Подпись колонки по связи: связанная запись приезжает рядом
       со ссылкой, и подписывается теми же полями показа, что и ячейка. */
    const slugs = field.relationId ? byId.get(field.relationId)?.viewFieldSlugs : undefined;

    return boardColumns({
      rows,
      tabs,
      slug: field.slug,
      unassigned: t("board.unassigned"),
      labelOf: (row, value) =>
        relationSelection(row, field, slugs).find((item) => item.guid === value)?.label ?? "",
    });
  }, [rows, field, byId, language, t]);

  /*
   * Порция приехала, а до низа доски по-прежнему меньше экрана — значит
   * прокручивать человеку будет нечего, и следующую порцию надо
   * заказывать самим. Порция за порцией, пока под сгибом не окажется
   * настоящий запас или строки не кончатся.
   *
   * Правило то же, что и у прокрутки, и это не случайно: «прокрутки нет
   * вовсе» — плохая проверка. Колонки растянуты на высоту доски и с
   * отступами дают полосу прокрутки в сотню пикселей на пустой доске,
   * то есть формально прокрутка есть, а грузить всё равно нужно.
   */
  const box = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const area = box.current;
    if (onEndReached && area && bottomGap(area) < END_GAP) onEndReached();
  }, [onEndReached, rows]);

  const drop = (column: string) => {
    const target = shown.find((item) => item.id === column);
    setDragging(null);
    setLifted(null);
    setOver(null);

    if (!dragging || !onMove || !target || over?.column !== column) return;

    /*
     * Место считается в списке БЕЗ перетаскиваемой карточки: внутри
     * своей же колонки она занимает строку, и без поправки бросок
     * на строку ниже давал бы номер на единицу больше нужного.
     */
    const from = target.rows.findIndex((row) => row.guid === dragging);
    const rest = from >= 0 ? target.rows.filter((row) => row.guid !== dragging) : target.rows;
    const at = from >= 0 && from < over.index ? over.index - 1 : over.index;
    // Бросили туда же, откуда взяли: запрос ничего не изменит.
    if (from === at) return;

    onMove(dragging, {
      [field.slug]: groupValue(field, column),
      [BOARD_ORDER]: boardOrderAt(rest, at),
    });
  };

  return (
    <div
      ref={box}
      className="flex-1 overflow-auto bg-bg px-3 pb-3"
      onScroll={(event: UIEvent<HTMLDivElement>) => {
        if (onEndReached && bottomGap(event.currentTarget) < END_GAP) onEndReached();
      }}
    >
      {/* Колонки одной высоты: у короткой шапка иначе уезжает вверх
          вместе с её карточками, пока соседние стоят на месте. */}
      <div className="flex min-h-full gap-3 pt-3">
        {shown.map((column) => {
          const option = field.options.get(column.id);
          const color: ChipColor = option?.color ? hexToChipColor(option.color) : "gray";
          const tint = CHIP_SURFACE[color];
          const isOver = over?.column === column.id;

          return (
            <section key={column.id || NO_GROUP_KEY} className="group/column flex w-72 shrink-0 flex-col">
              {/*
               * Шапка липнет к верху доски. Подложка двойная: цвет
               * колонки полупрозрачный, и сквозь него просвечивали бы
               * проезжающие карточки. Нижний слой — фон страницы,
               * поверх него тот же цвет, что и у тела колонки.
               *
               * Скругление — только у верхнего слоя: у прямоугольной
               * подложки углы закрашены фоном страницы, и колонка
               * выглядит одинаково что на месте, что под прокруткой.
               * Скругли её тоже — и в уголках была бы видна проезжающая
               * карточка, то есть радиус на глазах пропадал бы.
               */}
              <div className="sticky top-0 z-10 shrink-0 bg-bg">
                <header className={`flex h-11 items-center gap-2 rounded-t-xl px-2 ${tint}`}>
                  <Chip color={color} dot>
                    {column.label}
                  </Chip>

                  {/* Число загруженных: строки едут порциями, и пока едут
                      не все, у счётчика есть «+» — «столько уже здесь,
                      и это не всё». */}
                  <span className="shrink-0 text-xs text-fg-muted tabular-nums">
                    {column.rows.length}
                    {hasMore ? "+" : ""}
                  </span>

                  {/* Кнопка появляется на наведении: она нужна раз в день,
                      а рябит в шапке каждой колонки постоянно. С клавиатуры
                      доступна по-прежнему — фокус её показывает. */}
                  {onAddCard && (
                    <button
                      type="button"
                      onClick={() => onAddCard(column.id)}
                      aria-label={t("table.addRow")}
                      title={t("table.addRow")}
                      className="ml-auto grid size-6 shrink-0 place-items-center rounded-md text-fg-subtle opacity-0 transition hover:bg-surface hover:text-fg focus-visible:opacity-100 group-hover/column:opacity-100"
                    >
                      <Icon as={IconPlus} size={16} />
                    </button>
                  )}
                </header>
              </div>

              <div
                className={`flex min-h-24 flex-1 flex-col gap-2 rounded-b-xl px-2 pb-2 ${tint}`}
                onDragOver={(event) => {
                  if (!dragging) return;
                  // Без preventDefault браузер считает область запрещённой
                  // для броска и курсор показывает перечёркнутый круг.
                  event.preventDefault();
                  setOver((current) =>
                    current?.column === column.id
                      ? current
                      : { column: column.id, index: column.rows.length },
                  );
                }}
                onDrop={(event) => {
                  event.preventDefault();
                  drop(column.id);
                }}
              >
                {column.rows.map((row, index) => (
                  /* Карточку, которую тащат, из списка убираем: её место
                     занимает пустая рамка там, куда её бросят, и список
                     раздвигается ровно так, как он будет выглядеть после
                     броска. */
                  <div key={row.guid ?? index} className={lifted === row.guid ? "hidden" : ""}>
                    {isOver && over.index === index && <Placeholder height={height} />}

                    <article
                      draggable={Boolean(onMove) && editing !== row.guid}
                      onDragStart={(event: DragEvent<HTMLElement>) => {
                        // Firefox не начинает перетаскивание без данных
                        // в буфере, даже если они никому не нужны.
                        event.dataTransfer.setData("text/plain", row.guid ?? "");
                        event.dataTransfer.effectAllowed = "move";
                        /*
                         * Снимок карточки под курсором делается СЕЙЧАС:
                         * через миг она спрячется, и браузер утащил бы
                         * пустоту. Смещение — чтобы карточка держалась
                         * там же, где её взяли, а не прыгала углом
                         * к курсору.
                         */
                        const area = event.currentTarget.getBoundingClientRect();
                        event.dataTransfer.setDragImage(
                          event.currentTarget,
                          event.clientX - area.left,
                          event.clientY - area.top,
                        );
                        setHeight(area.height);

                        const guid = row.guid ?? null;
                        setDragging(guid);
                        // Место под карточкой освобождаем кадром позже —
                        // см. `lifted`.
                        requestAnimationFrame(() => setLifted(guid));
                      }}
                      onDragEnd={() => {
                        setDragging(null);
                        setLifted(null);
                        setOver(null);
                      }}
                      onDragOver={(event) => {
                        if (!dragging) return;
                        event.preventDefault();
                        // Иначе сработает обработчик колонки и место
                        // броска всегда оказывалось бы в конце списка.
                        event.stopPropagation();

                        const area = event.currentTarget.getBoundingClientRect();
                        const after = event.clientY > area.top + area.height / 2;
                        const next = index + (after ? 1 : 0);

                        setOver((current) =>
                          current?.column === column.id && current.index === next
                            ? current
                            : { column: column.id, index: next },
                        );
                      }}
                      /* Раскрытая карточка щелчком не открывается: в ней
                         правят поля, и переход в карточку записи посреди
                         правки — потеря места. */
                      onClick={() =>
                        editing !== row.guid && row.guid && onOpenRow(row.guid)
                      }
                      /* select-none: без него перетаскивание начинается
                         с выделения текста карточки, и вместо неё
                         в руке оказывается кусок текста. */
                      className={`group/card relative rounded-xl border bg-surface p-2.5 shadow-xs transition-colors select-none ${
                        editing === row.guid
                          ? "border-accent"
                          : "cursor-pointer border-border hover:border-border-strong"
                      }`}
                    >
                      {onEdit && row.guid && (
                        /*
                         * Правка на месте, не открывая карточку записи:
                         * поменять статус и срок — это два щелчка, а не
                         * переход туда и обратно. Кнопка появляется
                         * на наведении, чтобы не спорить с содержимым.
                         */
                        <button
                          type="button"
                          onClick={(event) => {
                            event.stopPropagation();
                            setEditing((current) =>
                              current === row.guid ? null : (row.guid ?? null),
                            );
                          }}
                          aria-label={t(editing === row.guid ? "action.close" : "action.edit")}
                          title={t(editing === row.guid ? "action.close" : "action.edit")}
                          className={`absolute top-1.5 right-1.5 z-10 grid size-6 place-items-center rounded-md border border-border bg-surface text-fg-muted transition hover:bg-surface-hover hover:text-fg ${
                            editing === row.guid
                              ? ""
                              : "opacity-0 focus-visible:opacity-100 group-hover/card:opacity-100"
                          }`}
                        >
                          <Icon as={editing === row.guid ? IconX : IconPencil} size={14} />
                        </button>
                      )}

                      <Card
                        columns={columns}
                        row={row}
                        tableSlug={tableSlug}
                        relations={byId}
                        locale={locale}
                        language={language}
                        {...(editing === row.guid && onEdit ? { onEdit } : {})}
                        {...(onSettings ? { onSettings } : {})}
                      />
                    </article>
                  </div>
                ))}

                {isOver && over.index >= column.rows.length && <Placeholder height={height} />}

                {/* Кнопка внизу колонки — как в старой админке: карточку
                    заводят в конец списка, а не «где-нибудь». Заодно она
                    и есть дно пустой колонки, куда можно бросить. */}
                {onAddCard && (
                  <button
                    type="button"
                    onClick={() => onAddCard(column.id)}
                    className="flex h-8 shrink-0 items-center gap-1.5 rounded-md px-2 text-sm text-fg-subtle transition-colors hover:bg-surface hover:text-fg"
                  >
                    <Icon as={IconPlus} size={16} />
                    {t("table.addRow")}
                  </button>
                )}
              </div>
            </section>
          );
        })}
      </div>
    </div>
  );
}

/**
 * Место, куда встанет карточка, — её же размером.
 *
 * Не тонкая линия между карточками: линию приходится выцеливать, а
 * прямоугольник в полный рост показывает результат броска до того, как
 * кнопку отпустили, — соседи уже раздвинулись.
 */
function Placeholder({ height }: { height: number }) {
  return (
    <div
      aria-hidden
      style={{ height }}
      className="mb-2 rounded-xl border-2 border-dashed border-accent bg-accent-subtle"
    />
  );
}

/**
 * Содержимое карточки: обложка, заголовок и значения остальных колонок
 * view — подряд, без подписей.
 *
 * Подписи — всплывающей подсказкой, как в старой админке
 * (BoardCardRowGenerator, `rowHint`): в колонке шириной 288px подпись
 * съедает половину строки, а карточка и так читается — «12.03.2026»
 * и «Иванов» не путаются между собой. Пустые значения пропускаются:
 * колонок во view бывает три десятка, и карточка из прочерков не
 * говорит ничего, зато занимает экран.
 *
 * Заголовок — первая колонка view, целиком и с переносами: у задачи
 * это её название, и обрезать его многоточием значит спрятать
 * единственное, ради чего на карточку смотрят.
 *
 * С `onEdit` карточка раскрыта на правку, и тогда всё наоборот: поля
 * показаны ВСЕ, включая пустые, — иначе незаполненное поле нечем
 * заполнить, — у каждого свой значок, а щелчок открывает тот же
 * редактор, что в таблице и в карточке записи.
 */
function Card({
  columns,
  row,
  tableSlug,
  relations,
  locale,
  language,
  onEdit,
  onSettings,
}: {
  columns: Field[];
  row: Item;
  tableSlug: string;
  relations: Map<string, Relation>;
  locale: string;
  language: string;
  onEdit?: ((guid: string, slug: string, value: unknown) => void) | undefined;
  onSettings?: ((field: Field, anchor: DOMRect) => void) | undefined;
}) {
  const { t } = useTranslation();
  /** Открытый редактор поля: его слаг и место, у которого он всплыл. */
  const [active, setActive] = useState<{ slug: string; anchor: DOMRect } | null>(null);
  const activeField = columns.find((item) => item.slug === active?.slug);

  /*
   * Фотография — обложкой во всю ширину, как в старой админке
   * (BoardPhotoGenerator). Только первая непустая и только PHOTO:
   * MULTI_IMAGE — это набор картинок, и он остаётся строкой значений.
   */
  const cover = columns.find((item) => item.type === "PHOTO" && !isBlank(row[item.slug]));
  const [first, ...rest] = columns.filter((item) => item !== cover);
  /* Заголовка нет — первая колонка у этой записи не заполнена. Строка
     с прочерком вместо названия занимает место и ничего не сообщает;
     в раскрытой карточке она, наоборот, нужна — её же и заполняют. */
  const title = first && (onEdit || !isBlank(row[first.slug])) ? first : undefined;
  const values = onEdit ? rest : rest.filter((item) => !isBlank(row[item.slug]));

  const cell = (item: Field, wrap = false) => (
    <Cell
      field={item}
      row={row}
      tableSlug={tableSlug}
      relations={relations}
      locale={locale}
      language={language}
      wrap={wrap}
    />
  );

  /** Значение или, если его нет, подпись поля бледным — как в Notion. */
  const valueOf = (item: Field) =>
    isBlank(row[item.slug]) ? (
      <span className="truncate text-fg-subtle">
        {localized(item.labels, language, item.label)}
      </span>
    ) : (
      cell(item)
    );

  const open = (item: Field, element: HTMLElement) => {
    /*
     * У BUTTON значения нет — редактировать нечего, а щелчок по ней
     * и так зовёт функцию. Поле, закрытое от правки, тоже открывать
     * незачем: редактор не откроется, а место щелчка запомнится.
     */
    if (cellKind(item.type) === "button") return;
    setActive({ slug: item.slug, anchor: element.getBoundingClientRect() });
  };

  return (
    <>
      {cover && (
        /*
         * Щелчок по обложке открывает саму фотографию, а не запись:
         * на картинку жмут, чтобы её разглядеть. Остальная карточка
         * по-прежнему открывает карточку записи.
         *
         * `draggable={false}` обязателен: иначе с фотографии начинается
         * перетаскивание САМОЙ фотографии — браузер тащит её адрес,
         * а карточка остаётся на месте.
         */
        <button
          type="button"
          onClick={(event) => {
            event.stopPropagation();
            openPreview([String(row[cover.slug])], 0, "image");
          }}
          className="mb-2 block w-full"
        >
          <img
            src={String(row[cover.slug])}
            alt=""
            loading="lazy"
            draggable={false}
            className="h-28 w-full rounded-lg object-cover"
          />
        </button>
      )}

      {title &&
        (onEdit ? (
          <button
            type="button"
            onClick={(event) => open(title, event.currentTarget)}
            className="mb-1.5 flex min-h-7 w-full items-center rounded-md px-1 pr-7 text-left text-sm font-medium transition-colors hover:bg-surface-hover"
          >
            {isBlank(row[title.slug]) ? (
              <span className="truncate text-fg-subtle">{t("board.untitled")}</span>
            ) : (
              cell(title, true)
            )}
          </button>
        ) : (
          <div className="mb-1.5 pr-6 text-sm font-medium">{cell(title, true)}</div>
        ))}

      <div className="flex flex-col gap-1">
        {values.map((item) =>
          onEdit ? (
            <Tooltip key={item.id} label={item.slug}>
              <button
                type="button"
                onClick={(event) => open(item, event.currentTarget)}
                className="flex min-h-7 w-full min-w-0 items-center gap-2 rounded-md px-1 text-left text-xs text-fg-muted transition-colors hover:bg-surface-hover"
              >
                <Icon as={fieldIcon(item.type)} size={14} className="shrink-0 text-fg-subtle" />
                {valueOf(item)}
              </button>
            </Tooltip>
          ) : (
            /* Чьё это значение — по наведению: на карточке видно
               значение, а слаг ещё и то имя, которым поле зовут в API. */
            <Tooltip key={item.id} label={item.slug}>
              <div className="flex min-w-0 items-center px-1 text-xs text-fg-muted">
                {cell(item)}
              </div>
            </Tooltip>
          ),
        )}
      </div>

      {/* Тот же редактор, что в таблице и в карточке записи: всплывает
          у поля, по которому щёлкнули, и пишет одно поле одной строки. */}
      {active && activeField && onEdit && (
        <ActiveCell
          key={active.slug}
          field={activeField}
          row={row}
          guid={row.guid}
          tableSlug={tableSlug}
          anchor={active.anchor}
          relations={relations}
          locale={locale}
          language={language}
          {...(onSettings ? { onSettings } : {})}
          onEdit={(value) => {
            if (row.guid) onEdit(row.guid, active.slug, value);
          }}
          onClose={() => setActive(null)}
        />
      )}
    </>
  );
}
