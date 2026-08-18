import { useMemo, useState } from "react";
import { IconEye, IconEyeOff, IconTrash } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import {
  DataGrid,
  FilterBar,
  GridFooter,
  GridSkeleton,
  ItemDrawer,
  TableToolbar,
  activeFilterCount,
  nextSorts,
  useCreateItem,
  useItems,
  useUpdateItem,
  type Filters,
  type RelationTab,
  type Sort,
} from "@/features/item";
import { localized, useTableSchema } from "@/features/table";
import { toast } from "@/shared/lib/toast";
import { Icon } from "@/shared/ui/icon";
import { Popover, PopoverItem, PopoverSeparator } from "@/shared/ui/popover";
import { ToolButton } from "@/shared/ui/tool-button";
import { columnKey, resolveColumnIds } from "../model/columns";

/**
 * Вкладка связи в карточке записи: строки ЧУЖОЙ таблицы, относящиеся
 * к открытой записи.
 *
 * Отбор — по колонке-ссылке из раскладки (`relation.relation_field_slug`),
 * а не по собранному имени `<слаг родительской таблицы>_id`. Старая
 * админка собирала имя сама, и у второй связи на ту же таблицу — где
 * колонка называется иначе — вкладка показывала не то.
 *
 * Живёт в features/view, а не в features/item: колонки вкладки — это
 * список id полей, и разворачивает его resolveColumnIds. Обратный импорт
 * (item → view) замкнул бы фичи в кольцо — ViewOptions уже импортирует
 * features/item.
 *
 * Сортировка, страница, поиск и отбор — своё состояние, а не адрес:
 * адрес занят основной таблицей, и второй набор тех же параметров
 * в нём означал бы либо префиксы у каждого ключа, либо путаницу «чья
 * это страница». Ссылка открывает вкладку с начала — это и есть та цена.
 */
export function RelationView({
  tab,
  parentGuid,
  parentValue,
  locale,
  language,
  canEdit,
  onColumns,
  onRemove,
}: {
  tab: RelationTab;
  /** guid открытой записи. По нему отбираются связанные строки. */
  parentGuid: string;
  /**
   * Значение колонки-ссылки в НАШЕЙ строке. Нужно вкладке обратного
   * направления: там связанная строка одна, и найти её можно только
   * по guid, который лежит у нас.
   */
  parentValue?: string | undefined;
  /** Локаль интерфейса: форматы дат и чисел. */
  locale: string;
  /** Язык данных: подписи полей и вариантов. */
  language: string;
  /** Право настраивать раскладку: без него колонки вкладки не правятся. */
  canEdit?: boolean;
  /** Новый набор колонок вкладки. Не задан — настройка недоступна. */
  onColumns?: ((columnIds: string[]) => void) | undefined;
  /** Убрать вкладку из карточки. Не задан — убирать нечем. */
  onRemove?: (() => void) | undefined;
}) {
  const { t } = useTranslation();

  /*
   * Настройки связей запрашиваются по показанным колонкам. У вкладки
   * без своих колонок показаны ВСЕ поля, поэтому и список ограничивать
   * нечем: иначе колонка-связь во вкладке осталась бы пустой — её
   * view_fields никто бы не загрузил.
   */
  const { schema, isLoading: schemaLoading } = useTableSchema(
    tab.tableSlug,
    tab.columnIds.length ? tab.columnIds : undefined,
  );
  /*
   * Колонки вкладки. Пустой список — обычное дело: бэкенд заводит вкладку
   * вместе со связью и колонок в неё не кладёт. Пустая вкладка ничего
   * не сообщает, поэтому по умолчанию показываются все поля чужой
   * таблицы, а сузить их можно настройкой вкладки.
   */
  const columns = useMemo(
    () =>
      tab.columnIds.length ? resolveColumnIds(tab.columnIds, schema.fields) : schema.fields,
    [tab.columnIds, schema.fields],
  );

  const [sorts, setSorts] = useState<Sort[]>([]);
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(LIMIT);
  const [search, setSearch] = useState("");
  const [own, setOwn] = useState<Filters>({});
  const [filtersOpen, setFiltersOpen] = useState(false);
  /** Раскрытая связанная строка. Пусто — открыт список. */
  const [openGuid, setOpenGuid] = useState<string | null>(null);

  /*
   * `contains` — не поиск подстроки, а форма записи: в теле get-list это
   * голое значение рядом со слагом (`{author_id: "<guid>"}`), и ровно так
   * его шлёт старая админка. Отдельной операции «равно строке» в наших
   * фильтрах нет: `equals` занят булевыми.
   *
   * Отбор человека домешивается сверху, но связь с открытой записью
   * перекрыть нельзя: вкладка без неё показала бы чужую таблицу целиком.
   *
   * Куда смотрит связь, решает direction:
   *   incoming — чужая колонка со ссылкой на нас;
   *   outgoing — наша колонка со ссылкой на чужую строку, и тогда
   *              отбираем чужую таблицу по её же guid.
   */
  const link = tab.direction === "incoming" ? tab.fieldSlug : "guid";
  const value = tab.direction === "incoming" ? parentGuid : (parentValue ?? "");

  const filters = useMemo(
    () => ({ ...own, [link]: { op: "contains" as const, values: [value] } }),
    [own, link, value],
  );

  const { page: rows, isLoading } = useItems(value ? tab.tableSlug : undefined, {
    limit,
    page,
    sorts,
    filters,
    search,
  });

  const update = useUpdateItem(tab.tableSlug);
  const create = useCreateItem(tab.tableSlug);

  /*
   * Выделение вкладке не нужно: массовых действий над связанными
   * строками здесь нет, а DataGrid без него не собирается. Пустое
   * и неизменное — колонка с флажками остаётся, но всегда пуста.
   */
  const [selected, setSelected] = useState<Set<string>>(() => new Set());

  /*
   * У обратной связи ссылка лежит в нашей строке, и пока её не задали,
   * показывать нечего: отбор по пустому guid вернул бы чужую таблицу
   * целиком.
   */
  if (!value) return <p className="p-6 text-sm text-fg-muted">{t("drawer.noRelated")}</p>;
  if (schemaLoading || isLoading) return <GridSkeleton columns={columns.length || 4} />;
  if (!columns.length) return <p className="p-6 text-sm text-fg-muted">{t("table.noColumns")}</p>;

  const openRow = rows.rows.find((item) => item.guid === openGuid);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {/* Своя панель инструментов: у вкладки свой список, и искать
          в нём приходится ровно так же, как в основной таблице. */}
      <div className="flex h-9 shrink-0 items-center justify-end gap-0.5 border-b border-border px-2">
        <TableToolbar
          tableSlug={tab.tableSlug}
          columns={columns}
          language={language}
          sorts={sorts}
          onSorts={(next) => {
            setSorts(next);
            setPage(1);
          }}
          filtersOpen={filtersOpen}
          filterCount={activeFilterCount(own)}
          onToggleFilters={() => setFiltersOpen((value) => !value)}
          search={search}
          onSearch={(next) => {
            setSearch(next);
            setPage(1);
          }}
        />

        {onColumns && canEdit && (
          <TabColumns
            fields={schema.fields}
            shown={columns}
            language={language}
            onChange={onColumns}
            {...(onRemove ? { onRemove } : {})}
          />
        )}
      </div>

      {filtersOpen && (
        <FilterBar
          columns={columns}
          language={language}
          filters={own}
          sorts={sorts}
          onFilters={(next) => {
            setOwn(next);
            setPage(1);
          }}
          onSorts={(next) => {
            setSorts(next);
            setPage(1);
          }}
        />
      )}

      <DataGrid
        tableSlug={tab.tableSlug}
        columns={columns}
        rows={rows.rows}
        relations={schema.relations}
        locale={locale}
        language={language}
        selected={selected}
        onSelect={setSelected}
        sorts={sorts}
        onSort={(field, direction) => {
          setSorts(direction ? [{ field, direction }] : nextSorts(sorts, field));
          setPage(1);
        }}
        onEdit={(guid, slug, value) => update.mutate({ guid, slug, value })}
        // Связанная строка раскрывается на месте, поверх вкладки:
        // у чужой таблицы своего экрана в этом меню нет, а посмотреть
        // на неё целиком нужно чаще, чем перейти в её таблицу.
        onOpenRow={setOpenGuid}
        /*
         * Ссылка на открытую запись проставляется сама: связанную строку
         * заводят ИЗ карточки, и заполнять её вручную значит предложить
         * человеку выбрать ту самую запись, из которой он смотрит.
         */
        {...(tab.canCreate
          ? {
              creating: create.isPending,
              onCreate: (values: Record<string, unknown>, done: () => void) =>
                create.mutate(
                  { ...values, [tab.fieldSlug]: parentGuid },
                  {
                    onSuccess: () => {
                      done();
                      toast.success(t("table.rowCreated"));
                    },
                  },
                ),
            }
          : {})}
      />

      <GridFooter
        page={page}
        limit={limit}
        total={rows.count}
        selectedCount={0}
        deleting={false}
        onPage={setPage}
        onLimit={(next) => {
          setLimit(next);
          setPage(1);
        }}
        onDeleteSelected={() => {}}
      />

      {/*
        Карточка связанной строки. Раскладки у чужой таблицы в этом меню
        нет, поэтому поля идут списком в порядке колонок вкладки —
        и это честнее, чем показать её раскладку из другого меню.
      */}
      {openRow && (
        <ItemDrawer
          key={openGuid}
          tableSlug={tab.tableSlug}
          columns={columns}
          row={openRow}
          relations={schema.relations}
          locale={locale}
          language={language}
          languages={[]}
          sections={[]}
          heading=""
          onEdit={(guid, slug, value) => update.mutate({ guid, slug, value })}
          onClose={() => setOpenGuid(null)}
        />
      )}
    </div>
  );
}

/**
 * Колонки вкладки. Хранятся в раскладке карточки, поэтому и правятся
 * ею же — отдельного view у вкладки нет.
 *
 * Список полей — ВСЕ поля чужой таблицы: скрытую колонку иначе неоткуда
 * вернуть.
 */
function TabColumns({
  fields,
  shown,
  language,
  onChange,
  onRemove,
}: {
  fields: Parameters<typeof resolveColumnIds>[1];
  shown: Parameters<typeof resolveColumnIds>[1];
  language: string;
  onChange: (columnIds: string[]) => void;
  /** Убрать вкладку целиком. Не задан — пункта нет. */
  onRemove?: (() => void) | undefined;
}) {
  const { t } = useTranslation();
  const visible = new Set(shown.map((field) => field.id));

  return (
    <Popover
      align="end"
      trigger={({ open, toggle }) => (
        <ToolButton icon={IconEye} label={t("view.columns")} open={open} onClick={toggle} />
      )}
    >
      {(close) => (
        <div className="max-h-72 w-64 overflow-y-auto">
          <p className="px-2 py-1 text-2xs text-fg-subtle">{t("drawer.tabColumnsHint")}</p>

          {fields.map((field) => {
            const on = visible.has(field.id);

            return (
              <PopoverItem
                key={field.id}
                active={on}
                icon={
                  <Icon
                    as={on ? IconEye : IconEyeOff}
                    size={16}
                    className={`shrink-0 ${on ? "" : "text-fg-subtle"}`}
                  />
                }
                onClick={() =>
                  onChange(
                    on
                      ? shown.filter((item) => item.id !== field.id).map(columnKey)
                      : [...shown, field].map(columnKey),
                  )
                }
              >
                {localized(field.labels, language, field.label)}
              </PopoverItem>
            );
          })}

          {/* Убрать вкладку — здесь же: заводят её в полосе вкладок,
              а снимают там, где настраивают. */}
          {onRemove && (
            <>
              <PopoverSeparator />
              <PopoverItem
                danger
                icon={<Icon as={IconTrash} size={16} className="shrink-0" />}
                onClick={() => {
                  onRemove();
                  close();
                }}
              >
                {t("drawer.removeTab")}
              </PopoverItem>
            </>
          )}
        </div>
      )}
    </Popover>
  );
}

/** Строк на странице вкладки. Меньше, чем у таблицы: места под неё меньше. */
const LIMIT = 10;
