import { useMemo } from "react";
import type { Filters } from "@/features/item";
import { localized, useRelationRows, type Field, type Relation } from "@/features/table";
import type { View } from "../model/types";

/**
 * Раскладка таблицы вкладками (`group_fields`).
 *
 * Не группировка строк: список сужается до одного значения выбранного
 * поля, и остальные строки на экран не приезжают вовсе. Поэтому вкладка
 * — это не украшение, а часть отбора: она домешивается к запросу ровно
 * там же, где отбор по умолчанию.
 *
 * Вкладки бывают двух происхождений, и это единственное, что здесь
 * сложного:
 *
 *   варианты поля  (PICK_LIST, MULTISELECT, STATUS) — уже лежат в схеме,
 *                  запроса нет вовсе;
 *   строки связи   (LOOKUP, LOOKUPS) — их приходится спрашивать,
 *                  и до ответа спрашивать строки таблицы рано.
 */

/** Поля, по которым таблицу можно разложить вкладками. Как в старой админке. */
export const TAB_GROUP_TYPES = new Set([
  "LOOKUP",
  "LOOKUPS",
  "PICK_LIST",
  "MULTISELECT",
  "STATUS",
]);

/**
 * Сколько строк связи показывать вкладками.
 *
 * ponytail: потолок, а не пагинация. Полсотни вкладок — это уже полоса
 * прокрутки во всю ширину; если у кого-то справочник больше, вкладки ему
 * не подходят вовсе, и лечится это выбором другого поля, а не догрузкой.
 */
const TAB_LIMIT = 50;

export type TabGroupTab = { id: string; label: string };

export type TabGroup = {
  /** Поле раскладки. undefined — вкладок нет, всё остальное пусто. */
  field: Field | undefined;
  tabs: TabGroupTab[];
  /** Открытая вкладка. Пусто — раскладки нет или у поля нет вариантов. */
  activeId: string;
  /** Что домешать к отбору. Пустой объект — ничего. */
  filters: Filters;
  /**
   * Варианты ещё едут: спрашивать строки рано. Без этого таблица успевает
   * приехать без вкладочного условия и тут же перезапрашивается с ним —
   * два запроса вместо одного и мигание чужими строками.
   */
  pending: boolean;
};

const NO_TAB_GROUP: TabGroup = { field: undefined, tabs: [], activeId: "", filters: {}, pending: false };

/** Поле раскладки. Ключ тот же, что и у колонок: у связи это id связи. */
export function tabGroupField(view: View | undefined, fields: Field[]): Field | undefined {
  if (!view?.tabGroupId) return undefined;

  return fields.find(
    (field) => field.id === view.tabGroupId || field.relationId === view.tabGroupId,
  );
}

export function useTabGroup({
  view,
  fields,
  relations,
  language,
  selected,
  enabled = true,
}: {
  view: View | undefined;
  /** ВСЕ поля таблицы: разложить можно и по скрытой колонке. */
  fields: Field[];
  relations: Relation[];
  /** Язык ДАННЫХ: подписи вариантов хранятся на языках проекта. */
  language: string;
  /** Вкладка из адреса. Не нашлась среди вариантов — берётся первая. */
  selected: string | undefined;
  /**
   * Нужны ли вкладки вообще. Выключено — строки связи не спрашиваются:
   * ту же настройку читает доска, но колонки она собирает из данных,
   * и полсотни чужих строк ей ни к чему.
   */
  enabled?: boolean;
}): TabGroup {
  const field = tabGroupField(view, fields);
  const relation = field?.relationId
    ? relations.find((item) => item.id === field.relationId)
    : undefined;

  /*
   * Строки чужой таблицы — только когда раскладка действительно по связи.
   * Пустой слаг выключает запрос: у поля с вариантами спрашивать нечего.
   */
  const { rows, isLoading } = useRelationRows({
    tableSlug: enabled ? (relation?.toSlug ?? "") : "",
    viewFields: relation?.viewFields ?? [],
    search: "",
    limit: TAB_LIMIT,
  });

  return useMemo(() => {
    if (!field) return NO_TAB_GROUP;

    const tabs: TabGroupTab[] = relation
      ? rows.map((row) => ({ id: row.guid, label: row.label || row.guid }))
      : [...field.options.values()].map((option) => ({
          id: option.value,
          label: localized(option.labels, language, option.label || option.value),
        }));

    /*
     * Открыта та, что в адресе, иначе первая: «все записи» вкладкой
     * не бывает — раскладка на то и раскладка, что показывает часть.
     * Так же ведёт себя и старая админка (Grid.jsx, setGroupTab).
     */
    const activeId = tabs.find((tab) => tab.id === selected)?.id ?? tabs[0]?.id ?? "";

    return {
      field,
      tabs,
      activeId,
      filters: activeId ? { [field.slug]: tabFilter(field, activeId) } : {},
      pending: Boolean(relation) && isLoading,
    };
  }, [field, relation, rows, language, selected, isLoading]);
}

/**
 * Условие одной вкладки.
 *
 * У MULTISELECT в колонке лежит список, и отбирать по нему нужно
 * «любое из» — списком же (так это делает и старая админка,
 * Grid.jsx: `[${groupTab.value}]`). У остальных значение одно,
 * и сравнение точное: `contains` нашёл бы «renew» по «new».
 */
function tabFilter(field: Field, value: string): Filters[string] {
  return field.type === "MULTISELECT" ? { op: "any", values: [value] } : { op: "is", values: [value] };
}
