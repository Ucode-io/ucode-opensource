import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  DataGrid,
  GridFooter,
  GridSkeleton,
  nextSorts,
  useCreateItem,
  useItems,
  useUpdateItem,
  type RelationTab,
  type Sort,
} from "@/features/item";
import { useTableSchema } from "@/features/table";
import { toast } from "@/shared/lib/toast";
import { resolveColumnIds } from "../model/columns";

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
 * Сортировка и страница — своё состояние, а не адрес: адрес занят
 * основной таблицей, и второй набор тех же параметров в нём означал бы
 * либо префиксы у каждого ключа, либо путаницу «чья это страница».
 * Ссылка открывает вкладку с начала — это и есть та цена.
 */
export function RelationView({
  tab,
  parentGuid,
  locale,
  language,
  onOpenRow,
}: {
  tab: RelationTab;
  /** guid открытой записи. По нему отбираются связанные строки. */
  parentGuid: string;
  /** Локаль интерфейса: форматы дат и чисел. */
  locale: string;
  /** Язык данных: подписи полей и вариантов. */
  language: string;
  /** Раскрыть связанную строку. Нет — строки только читаются на месте. */
  onOpenRow?: ((guid: string) => void) | undefined;
}) {
  const { t } = useTranslation();

  const { schema, isLoading: schemaLoading } = useTableSchema(tab.tableSlug, tab.columnIds);
  const columns = useMemo(
    () => resolveColumnIds(tab.columnIds, schema.fields),
    [tab.columnIds, schema.fields],
  );

  const [sorts, setSorts] = useState<Sort[]>([]);
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(LIMIT);

  /*
   * `contains` — не поиск подстроки, а форма записи: в теле get-list это
   * голое значение рядом со слагом (`{author_id: "<guid>"}`), и ровно так
   * его шлёт старая админка. Отдельной операции «равно строке» в наших
   * фильтрах нет: `equals` занят булевыми.
   */
  const filters = useMemo(
    () => ({ [tab.fieldSlug]: { op: "contains" as const, values: [parentGuid] } }),
    [tab.fieldSlug, parentGuid],
  );

  const { page: rows, isLoading } = useItems(parentGuid ? tab.tableSlug : undefined, {
    limit,
    page,
    sorts,
    filters,
  });

  const update = useUpdateItem(tab.tableSlug);
  const create = useCreateItem(tab.tableSlug);

  /*
   * Выделение вкладке не нужно: массовых действий над связанными
   * строками здесь нет, а DataGrid без него не собирается. Пустое
   * и неизменное — колонка с флажками остаётся, но всегда пуста.
   */
  const [selected, setSelected] = useState<Set<string>>(() => new Set());

  if (schemaLoading || isLoading) return <GridSkeleton columns={columns.length || 4} />;
  if (!columns.length) return <p className="p-6 text-sm text-fg-muted">{t("table.noColumns")}</p>;

  return (
    <div className="flex min-h-0 flex-1 flex-col">
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
        {...(onOpenRow ? { onOpenRow } : {})}
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
    </div>
  );
}

/** Строк на странице вкладки. Меньше, чем у таблицы: места под неё меньше. */
const LIMIT = 10;
