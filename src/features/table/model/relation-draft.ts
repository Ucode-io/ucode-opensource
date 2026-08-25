import type { CascadeStep } from "./cascade";
/**
 * Черновик связи — то, что человек набрал в панели поля, выбрав тип «Связь».
 *
 * Отдельно от FieldDraft, потому что связь и поле — разные сущности
 * с разными ручками: у поля есть тип, обязательность и проверка ввода,
 * у связи — целевая таблица и поля показа. Общий черновик означал бы
 * половину полей, не значащих ничего, и `if (isRelation)` в каждой форме.
 */
/**
 * Условие автофильтра: «показывать только те строки чужой таблицы,
 * у которых поле `fieldTo` совпадает со значением поля `fieldFrom`
 * в ЭТОЙ строке».
 *
 * Слаги, а не идентификаторы: в колонке `auto_filters` лежат именно
 * слаги, и по ним же собирается тело `get-list` (см. features/item,
 * `autoFilterValues`).
 */
export type AutoFilterPair = { fieldFrom: string; fieldTo: string };

export type RelationDraft = {
  /** Куда ведёт связь: слаг целевой таблицы. */
  toSlug: string;
  /**
   * Чем ограничен выбор строк. Пусто — выбирают из всей чужой таблицы.
   *
   * Это единственная настройка связи сверх целевой таблицы и полей
   * показа, которую мы даём задать, и она здесь потому, что без неё
   * зависимые списки не работают вовсе: город приходится искать среди
   * всех городов страны. Остальные колонки связи — `cascadings`,
   * `dynamic_tables`, `object_id_from_jwt` — уезжают обратно как пришли.
   */
  autoFilters: AutoFilterPair[];
  /**
   * Каскад: цепочка шагов сужения от дальнего предка к цели связи.
   * Пусто — каскада нет, связь выбирается одним списком. Формат
   * и разворот — в model/cascade.
   */
  cascade: CascadeStep[];
  /** Подпись колонки на языке ДАННЫХ. */
  label: string;
  /**
   * Что показывать вместо uuid: id полей ЦЕЛЕВОЙ таблицы.
   *
   * Обязателен хотя бы один. Связь без полей показа рабочей не бывает:
   * в ячейке нечего показать, и выбирать в ней приходится из столбика
   * одинаковых на вид идентификаторов. Подставить «первое текстовое»
   * за человека нельзя — см. [[Relation]] в CONTEXT, — поэтому
   * спрашиваем.
   */
  viewFieldIds: string[];
  /**
   * Чем заполнить колонку у НОВОЙ записи — «свой» строкой того, кто
   * её заводит. См. `Relation.selfDefault`: значений три и они
   * взаимоисключающие, поэтому это список, а не два флажка (в старой
   * админке их два, и поднять можно оба — выигрывает всё равно один).
   */
  selfDefault: "user" | "object" | null;
};

/**
 * Направление связи. Оно одно, и выбора у человека нет.
 *
 * Many2One — единственное, что таблица умеет не только показать, но и
 * ПРАВИТЬ: ссылка лежит колонкой в самой строке, и связать — это
 * обычная правка ячейки (см. features/item/model/relation, isLinkable).
 * У Many2Many связи лежат в третьей таблице, у One2Many — в чужих
 * строках; завести их отсюда значит выдать колонку, которую потом
 * нечем заполнить.
 *
 * Актуальный конструктор ucode делает так же: форма новой связи
 * спрашивает целевую таблицу и поля показа, выбора типа в ней нет.
 *
 * Many2Many к тому же нечем править и на стороне бэкенда: ручки
 * `PUT/DELETE /v2/items/many-to-many` для postgres-проектов отвечают
 * «does not implemented» (items.go:2232 в шлюзе), а обычный PUT пишет
 * только СВОЮ колонку `<чужая таблица>_ids` — вторая сторона остаётся
 * со старым списком. Симметрию бэкенд поддерживает ровно один раз,
 * при вставке (helper.AppendMany2Many). Чинить это из фронта значит
 * править чужие строки пачкой запросов; см. docs/backend-notes.md.
 *
 * ponytail: остальные направления существуют в базе и в старом экране
 * настроек — `Many2Many`, `Recursive`, `Many2Dynamic`. Вернутся сюда
 * вместе с ручкой, которая умеет их писать.
 */
export const RELATION_DIRECTION = "Many2One";

export const EMPTY_RELATION_DRAFT: RelationDraft = {
  toSlug: "",
  autoFilters: [],
  cascade: [],
  label: "",
  viewFieldIds: [],
  selfDefault: null,
};

/**
 * Черновик обратно в две колонки связи.
 *
 * Уезжают обе и всегда: UPDATE переписывает их безусловно
 * (`relation.go:2033`), и вернуть в теле только поднятую значило бы
 * оставить вторую в прежнем состоянии — то есть не снять её никогда.
 */
export function toSelfDefaultBody(value: RelationDraft["selfDefault"]) {
  return {
    is_user_id_default: value === "user",
    object_id_from_jwt: value === "object",
  };
}

/**
 * Пары автофильтра, как они лежат в связи, — в черновик.
 *
 * В базе это JSONB со змеиными именами (`{field_to, field_from}` —
 * pg_relation.proto:67), и разбирается он один раз здесь: половина
 * пары без второй половины условием не станет, поэтому недозаполненные
 * не переносятся вовсе.
 */
export function toAutoFilters(raw: Record<string, unknown>): AutoFilterPair[] {
  const pairs = raw["auto_filters"];
  if (!Array.isArray(pairs)) return [];

  return pairs
    .map((pair) => {
      const item = pair as Record<string, unknown> | null;
      return {
        fieldFrom: String(item?.["field_from"] ?? ""),
        fieldTo: String(item?.["field_to"] ?? ""),
      };
    })
    .filter((pair) => pair.fieldFrom && pair.fieldTo);
}

/** Обратно в тело запроса. Пустые строки формы в базу не уезжают. */
export function toAutoFiltersBody(pairs: AutoFilterPair[]): { field_from: string; field_to: string }[] {
  return pairs
    .filter((pair) => pair.fieldFrom && pair.fieldTo)
    .map((pair) => ({ field_from: pair.fieldFrom, field_to: pair.fieldTo }));
}

/**
 * Готова ли связь к отправке.
 *
 * Без целевой таблицы создавать нечем, без полей показа — незачем:
 * получится колонка, в которой не видно ничего, кроме идентификаторов.
 */
export function isRelationReady(draft: RelationDraft): boolean {
  return Boolean(draft.toSlug) && draft.viewFieldIds.length > 0;
}
