/** Сырые ответы схемы таблицы. Наружу фичи не выходят. */

/** GET /v2/fields/{slug} */
export type FieldDto = {
  id?: string;
  slug?: string;
  label?: string;
  type?: string;
  table_id?: string;
  is_visible?: boolean;
  required?: boolean;
  index?: number;
  /**
   * id связи у LOOKUP и LOOKUPS. Один и тот же смысл приходит под двумя
   * именами: в /v2/fields это `relation_field`, а внутри view_fields
   * связи — `relation_id`. Разворачивается один раз, в normalize.
   */
  relation_field?: string;
  relation_id?: string;
  /**
   * Мультиязычное поле. Колонка в БД, и только она: `enable_multi_language`
   * не существует, а `attributes.enable_multilanguage` бэкенд дописывает
   * сам при отдаче (object_builder.go:785) — читать её значит читать эхо.
   */
  enable_multilanguage?: boolean;
  attributes?: Record<string, unknown>;
};

export type FieldsResponseDto = {
  fields?: FieldDto[];
  count?: number;
};

/**
 * Вариант выбора, как он лежит в attributes. Форма зависит от типа поля:
 * у MULTISELECT значение в `slug`, у STATUS — в `value`. Подписи по
 * языкам данных россыпью ключей label_<short_name>, поэтому индексная
 * сигнатура, а не перечисление.
 */
export type OptionDto = {
  id?: string;
  slug?: string;
  value?: string;
  label?: string;
  color?: string;
  icon?: string;
  [key: string]: unknown;
};

/** GET /v2/relations/{slug} */
export type RelationDto = {
  id?: string;
  type?: string;
  /**
   * Стороны связи приходят объектами таблиц целиком — с подписью
   * и подписями по языкам данных в attributes. Именно они нужны там,
   * где у самой связи имени не задали: «Заказы» читается, `orders_id`
   * — нет.
   */
  table_from?: TableSideDto;
  table_to?: TableSideDto;
  /**
   * Колонка-связь в таблице `table_from` — у Many2One это
   * `<table_to>_id` (relation.go: `fieldFrom = data.TableTo + "_id"`).
   * Рядом с ней в строке приходит связанная запись, `<field_from>_data`.
   */
  field_from?: string;
  /** Поля целиком, а не слаги: бэкенд подставляет сюда весь объект поля. */
  view_fields?: FieldDto[];
  attributes?: Record<string, unknown>;
  [key: string]: unknown;
};

/** Сторона связи: таблица, как её отдаёт бэкенд внутри связи. */
export type TableSideDto = {
  id?: string;
  slug?: string;
  label?: string;
  attributes?: Record<string, unknown>;
};

export type RelationsResponseDto = {
  relations?: RelationDto[];
  count?: number;
};
