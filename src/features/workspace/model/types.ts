/** Компания — верхний уровень: у пользователя их может быть несколько. */
export type Company = { id: string; name: string };

/** Проект внутри компании. */
export type Project = { id: string; name: string };

/**
 * Язык ДАННЫХ проекта. Не язык интерфейса.
 *
 * Интерфейс переводится на ru/en/uz — это выбор пользователя для себя.
 * Языки данных задаёт проект, и на них хранятся значения мультиязычных
 * полей и подписи вариантов STATUS и MULTISELECT: ключ `label_<code>`,
 * где code — это `short_name` отсюда. В живом проекте это "en" и "cyr",
 * то есть с локалями интерфейса набор не совпадает вовсе.
 *
 * Старый ucode их смешивал: при загрузке насильно переключал i18n
 * на первый язык проекта, и интерфейс уезжал в язык данных.
 */
export type DataLanguage = {
  /** Ключ в данных: label_en, label_cyr. */
  code: string;
  name: string;
  /** Название на самом языке — для переключателя. */
  nativeName: string;
};

/**
 * Окружение внутри проекта (prod, dev). Переключение окружения — переход
 * с ротацией токенов, а не смена фильтра, см. docs/adr/0001.
 */
export type Environment = {
  id: string;
  name: string;
  projectId: string;
  color: string;
  isDefault: boolean;
};
