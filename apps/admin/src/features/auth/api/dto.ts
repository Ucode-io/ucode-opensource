/**
 * Сырые ответы бэкенда. Живут только здесь и наружу не выходят —
 * снаружи ходят типы из ../model/types.
 *
 * Поля необязательные не из осторожности, а потому что бэкенд собирает
 * ответ из proto с omitempty: отсутствие поля — норма, а не ошибка.
 */
export type TokenDto = {
  access_token: string;
  refresh_token: string;
  expires_at?: string;
  refresh_in_seconds?: number;
};

/**
 * Права на таблицу. Значения — СТРОКИ `'Yes' | 'No'`, а не булевы:
 * так они лежат в базе (record_permission, VARCHAR ... CHECK IN ('Yes','No'))
 * и так же уходят в ответ (login.proto, message RecordPermission).
 * Строка "No" в булевом контексте истинна — отсюда обязательный перевод
 * в normalize.
 */
export type PermissionDto = {
  table_slug?: string;
  read?: string;
  write?: string;
  update?: string;
  delete?: string;
  settings?: string;
  columns?: string;
  fix_column?: string;
  excel_menu?: string;
  view_create?: string;
  add_field?: string;
  group?: string;
  tab_group?: string;
  search_button?: string;
  field_filter?: string;
};

export type UserDto = {
  login?: string;
  email?: string;
  name?: string;
  photo_url?: string;
};

export type LoginResponseDto = {
  user_id?: string;
  user?: UserDto;
  environment_id?: string;
  login_table_slug?: string;
  token?: TokenDto;
  role?: { id?: string; name?: string };
  client_type?: { id?: string };
  permissions?: PermissionDto[];
  /**
   * Глобальные права роли: кнопки приложения, а не таблицы. Флаги
   * БУЛЕВЫ (своя таблица `global_permission`, своя форма в настройках
   * ролей), но рядом с ними лежит строковый `id` — отсюда `unknown`.
   * `false` в ответ не попадает вовсе — поля с omitempty
   * (session_service.pb.go, message GlobalPermission). Поэтому «нет
   * поля» здесь значит «выключено», а «нет объекта» — «мы не знаем».
   */
  global_permission?: Record<string, unknown>;
};

export type ProjectDataDto = {
  project_id?: string;
  company_id?: string;
  title?: string;
  name?: string;
};

/**
 * Одна connection. `view_slug` — имя поля, в котором лежит человекочитаемая
 * подпись записи, поэтому опции индексируются динамическим ключом.
 */
export type ConnectionDto = {
  guid?: string;
  table_slug?: string;
  view_slug?: string;
  options?: Record<string, unknown>[];
};

/**
 * Ответ /v3/multicompany/default-login после снятия конверта.
 * `response` — либо готовая сессия (проект один), либо список connection.
 */
export type DefaultLoginDto = {
  response?: LoginResponseDto | ConnectionDto[];
  project_data?: ProjectDataDto;
  client_type?: string;
  environment?: string;
  project?: string;
  user_id?: string;
};

/** Запрос POST /company — регистрация компании вместе с её первым пользователем. */
export type RegisterCompanyDto = {
  name: string;
  user_info: {
    login: string;
    email: string;
    password: string;
    phone?: string;
  };
};

/**
 * Ответ первого шага восстановления пароля (`ForgotPasswordResponse`
 * в auth-сервисе, api/models/user_v2.go).
 *
 * `email_found: false` при непустом `user_id` — это не отказ: логин нашли,
 * но почты у пользователя нет, и её сначала надо задать.
 */
export type ForgotPasswordDto = {
  user_id?: string;
  email_found?: boolean;
  sms_id?: string;
  email?: string;
};

/** Ответ проверки кода из письма. */
export type VerifyEmailDto = {
  verified?: boolean;
};

/** Ответ /v2/send-code-app: идентификатор отправленного кода. */
export type SendCodeDto = {
  sms_id?: string;
};
