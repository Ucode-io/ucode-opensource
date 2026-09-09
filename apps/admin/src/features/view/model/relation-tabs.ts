import type { Relation } from "@/features/table";
import { viewName, type View } from "./types";

/**
 * Вкладки связей в карточке записи.
 *
 * Источник — view пункта меню с `is_relation_view`, и это не деталь
 * хранения, а то, что видит человек. Ровно их показывает старая админка
 * (useViewsProps.jsx:363 — `item.type === "SECTION" || item.is_relation_view`),
 * и ровно их админ заводит руками в карточке.
 *
 * Почему не вкладки раскладки (`layout.tabs[type=relation]`), хотя они
 * тоже есть: бэкенд заводит такую вкладку САМ на каждую Many2One
 * (pkg/helper/relation.go:369) — вместе с безымянным view (там же, :335),
 * — и в карточке появлялись все связи таблицы разом, включая те, что
 * показывать никто не просил. Старая админка их не читает вовсе:
 * её фильтр по `relation.permission.view_permission` не проходит ни одна,
 * потому что прав эта ручка не отдаёт.
 *
 * Цена решения: в проекте, где relation view не заводили, карточка
 * останется без вкладок связей, пока их не добавят кнопкой «+».
 */
export type RelationTab = {
  /** id VIEW'а: он же в адресе (`?tab=`) и он же удаляется. */
  id: string;
  /**
   * Сам view. Настройки вкладки — это настройки view, и панель «⋯»
   * работает с ним напрямую: колонки, отбор, закрепление и имя живут
   * в его собственных колонках, а не в чужих attributes.
   */
  view: View;
  /** Связь, которую показывает вкладка. */
  relationId: string;
  label: string;
  /** Таблица, строки которой показывает вкладка. */
  tableSlug: string;
  /**
   * Колонка-ссылка. ГДЕ она лежит, говорит `direction`:
   *
   *   incoming — в чужой таблице, и в ней наш guid: строки отбираются
   *              по ней, новая строка ею же и привязывается;
   *   outgoing — в НАШЕЙ строке, и в ней guid чужой: вкладка показывает
   *              ровно одну строку, ту, на которую мы ссылаемся.
   */
  fieldSlug: string;
  direction: "incoming" | "outgoing";
  /** Колонки вкладки: ключи полей, как в `view.columns`. */
  columnIds: string[];
  /** Можно ли создавать связанные строки. */
  canCreate: boolean;
};

/**
 * View'шки пункта меню → вкладки карточки.
 *
 * Связь каждой вкладки ищется сначала по `relation_id`, и только потом
 * по слагу чужой таблицы: у view, заведённых старой админкой,
 * `relation_id` пуст — её форма его не шлёт, — а у наших и у заведённых
 * бэкендом он есть. Две связи на одну таблицу («склад отправитель»
 * и «склад получатель») различаются только им; по слагу выбрать из них
 * нельзя, и берётся первая — ровно так же промахивалась старая админка,
 * собирая имя колонки как `<слаг родителя>_id`.
 *
 * Вкладка без своей связи пропускается: отбирать строки нечем, и она
 * показала бы чужую таблицу целиком.
 */
export function relationTabs(
  views: View[],
  relations: Relation[],
  language: string,
): RelationTab[] {
  const byId = new Map(relations.map((relation) => [relation.id, relation]));

  return views
    .filter((view) => view.isRelationView)
    .sort((a, b) => a.order - b.order)
    .map((view) => {
      const relation =
        (view.relationId ? byId.get(view.relationId) : undefined) ??
        relations.find((item) => item.toSlug === view.relationTableSlug);

      if (!relation?.toSlug || !relation.linkField) return undefined;

      return {
        id: view.id,
        view,
        relationId: relation.id,
        /*
         * Имя: заданное админом, затем подпись чужой таблицы, затем
         * имя связи. Тот же порядок, что в старой админке
         * (useHeaderFilterProps.jsx, getViewName: `name_<язык>` →
         * `table_label` → тип). Тип мы не показываем: «TABLE» на вкладке
         * не говорит ничего.
         */
        label: viewName(view, language) || view.tableLabel || relation.title || relation.toSlug,
        tableSlug: relation.toSlug,
        fieldSlug: relation.linkField,
        direction: relation.direction,
        columnIds: view.columnIds,
        /*
         * Создавать можно только там, где ссылка лежит в чужой строке:
         * тогда новая строка привязывается тем же полем, которым
         * отбирается. У обратного направления привязка — это правка
         * НАШЕЙ строки, и делать её от имени «создать связанную»
         * значит менять запись, которую человек открыл смотреть.
         */
        canCreate: relation.direction === "incoming",
      };
    })
    .filter((tab): tab is RelationTab => Boolean(tab));
}

/**
 * Связи, которые годятся во вкладку карточки.
 *
 * Нужна одна колонка с guid: по ней вкладка отбирает строки, ею же
 * привязывает новую. У Many2Many колонка хранит массив (`<таблица>_ids`),
 * у Many2Dynamic рядом с идентификатором лежит ещё и слаг таблицы —
 * отбор по такой колонке вернёт не то. Старая админка выкидывала их
 * тем же списком (useViewCreatePopupProps.jsx, getTableRelations).
 *
 * Обратная сторона (мы ссылаемся на чужую строку) оставлена, хотя
 * старая её прятала: вкладка на одну строку — это карточка родителя
 * рядом с записью, и открывать её вручную незачем.
 */
export function tabbableRelations(relations: Relation[]): Relation[] {
  return relations.filter(
    (relation) => relation.linkField && !UNTABBABLE.has(relation.type),
  );
}

const UNTABBABLE = new Set(["Many2Many", "Many2Dynamic"]);
