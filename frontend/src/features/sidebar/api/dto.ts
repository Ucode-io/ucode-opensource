/** Сырой пункт меню: GET /v3/menus. Наружу не выходит. */
export type MenuDto = {
  id?: string;
  label?: string;
  icon?: string;
  type?: string;
  parent_id?: string;
  order?: number;
  table_id?: string;
  layout_id?: string;
  /** Какое чужое приложение показывает пункт типа MICROFRONTEND. */
  microfrontend_id?: string;
  is_static?: boolean;
  /** label_<locale>, website_link, link — свободный мешок бэкенда. */
  attributes?: Record<string, unknown>;
  /** Права роли на этот пункт приходят здесь. */
  data?: { permission?: Record<string, boolean> };
};

export type MenusResponseDto = {
  menus?: MenuDto[];
  count?: number;
};
