import { useState } from "react";
import { useTranslation } from "react-i18next";
import { localized, useTables } from "@/features/table";
import { useDataLanguages } from "@/features/workspace";
import { Dropdown } from "@/shared/ui/dropdown";

/**
 * Отбор по таблице — выбором из списка, а не набором подстроки: слаг
 * таблицы (`orders`, `user_addresses`) человек по памяти не наберёт,
 * а с опечаткой журнал молча покажет пусто.
 *
 * Список тот же, что у связей и у пункта меню, — `useTables`: та же
 * ручка, тот же поиск на сервере и та же догрузка по страницам.
 * Сотни таблиц в проекте — норма, поэтому целиком он не тянется.
 *
 * В отбор уходит СЛАГ: сервер сравнивает его по подстроке и со слагом,
 * и с подписью (`version_history.go:158`), но подпись переводится
 * и повторяется у разных таблиц, а слаг один.
 */
export function TableFilter({ value, onChange }: { value: string; onChange: (slug: string) => void }) {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  const { current: language } = useDataLanguages();
  const tables = useTables(search);

  const found = tables.items.map((table) => ({
    value: table.slug,
    label: localized(table.labels, language, table.label),
  }));

  const items = [
    // Первым пунктом — снять отбор: стереть выбор в списке больше нечем.
    ...(value ? [{ value: "", label: t("table.clearFilters") }] : []),
    /*
     * Выбранная таблица остаётся в списке, даже когда поиск её не нашёл:
     * иначе набранное в поиске чужое слово стирает подпись с кнопки,
     * и кажется, что отбора нет, — а он есть. Слага хватает: он же
     * и лежит в отборе.
     */
    ...(value && !found.some((item) => item.value === value)
      ? [{ value, label: value }]
      : []),
    ...found,
  ];

  return (
    <Dropdown
      value={value}
      items={items}
      placeholder={t("activity.table")}
      ariaLabel={t("activity.table")}
      searchPlaceholder={t("menuForm.tableSearch")}
      emptyText={t("menuForm.tableEmpty")}
      search={search}
      loading={tables.isLoading}
      hasMore={tables.hasMore}
      onSearch={setSearch}
      onLoadMore={tables.loadMore}
      onChange={onChange}
      size="sm"
    />
  );
}
