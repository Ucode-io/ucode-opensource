/**
 * Форма, в которой auth существует внутри приложения. Ответ бэкенда
 * приводится к ней один раз, в api/. Дальше по коду ходит только это.
 */
export type AuthSession = {
  userId: string;
  projectId: string;
  environmentId: string;
  roleId: string;
  clientTypeId: string;
  loginTableSlug: string;
  permissions: Permission[];
};

/**
 * Права роли на одну таблицу.
 *
 * Плоский набор булевых, хотя бэкенд отдаёт строки `'Yes' | 'No'`
 * (record_permission, VARCHAR с CHECK). Перевод — один раз, в api/normalize:
 * иначе `if (permission.read)` истинно и для `"No"`, и это ровно тот
 * случай, когда запрет читается как разрешение.
 *
 * Имена — бэкендовые, потому что второго словаря для них нет: это
 * колонки таблицы record_permission, и роль настраивается теми же
 * словами в настройках прав.
 */
export type Permission = {
  tableSlug: string;
  read: boolean;
  write: boolean;
  update: boolean;
  delete: boolean;
  /** Настройки view целиком: имя, тип, колонки, удаление вкладки. */
  settings: boolean;
  /** Список колонок. */
  columns: boolean;
  /** Закрепление колонок. */
  fixColumn: boolean;
  /** Импорт и выгрузка Excel. */
  excelMenu: boolean;
  /** Создание вкладки. */
  viewCreate: boolean;
  /** Добавление поля в таблицу. */
  addField: boolean;
  /** Группировка. */
  group: boolean;
  /** Группировка вкладками. */
  tabGroup: boolean;
  /** Поиск по таблице. */
  searchButton: boolean;
  /** Отбор по колонке из её меню. Подшапка с чипами правом не закрыта. */
  fieldFilter: boolean;
};

export type Credentials = {
  username: string;
  password: string;
};

/**
 * Вход по телефону: код из SMS вместо пароля. Ключи — те, что ждёт
 * default-login (`type: "phone"`, session_v2.go), поэтому snake_case:
 * объект уходит в тело запроса как есть — и при выборе connection'ов
 * тоже, вторым запросом в /v2/login.
 */
/**
 * Вход по коду — с телефона или с почты.
 *
 * Одна форма на оба: ручка входа различает их полем `type` и тем, в каком
 * ключе лежит получатель (session_v2.go, V3MultiCompanyLogin — случаи
 * WithPhone и WithEmail), а всё остальное у них общее: код, sms_id
 * и второй шаг с connection'ами.
 */
export type OtpCredentials =
  | { type: "phone"; phone: string; otp: string; sms_id: string }
  | { type: "email"; email: string; otp: string; sms_id: string };

/** Куда уходит код: в SMS или письмом. Так его называет send-code-app. */
export type OtpChannel = "PHONE" | "EMAIL";

/** Вариант выбора внутри одной Connection. */
export type ConnectionOption = {
  id: string;
  label: string;
};

/**
 * Connection — таблица, запись из которой пользователь должен выбрать,
 * чтобы войти. Появляется, когда один логин ведёт больше чем в одно
 * рабочее пространство.
 */
export type Connection = {
  id: string;
  tableSlug: string;
  options: ConnectionOption[];
};

/** Проект и окружение уже определены бэкендом — выбирается только запись. */
export type LoginContext = {
  clientTypeId: string;
  projectId: string;
  environmentId: string;
};

/** Что выбрано: id connection → id записи. */
export type ConnectionSelection = Record<string, string>;

export type LoginResult =
  | { kind: "session"; session: AuthSession }
  | { kind: "choose-connections"; connections: Connection[]; context: LoginContext };

export type Registration = {
  /** Название компании — рабочее пространство пользователя. */
  companyName: string;
  /** Логин: минимум 6 символов, требование бэкенда. */
  login: string;
  email: string;
  password: string;
};

/**
 * Чем закончился первый шаг восстановления пароля.
 *
 *   sent    код ушёл на почту, дальше он вводится;
 *   noEmail логин нашли, но почты у пользователя нет — её надо задать,
 *           и код уйдёт уже на неё;
 *   unknown такого логина нет.
 */
export type RecoveryStart =
  | { kind: "sent"; userId: string; smsId: string; email: string }
  | { kind: "noEmail"; userId: string }
  | { kind: "unknown" };

/**
 * Приглашение в проект: всё, кроме логина и пароля, приходит ссылкой.
 *
 * Ключи адреса именно такие, с дефисом в `project-id`: их читает
 * приглашающая сторона, и переименовать их нельзя, не сломав старые
 * письма.
 */
export type Invite = {
  projectId: string;
  environmentId: string;
  roleId: string;
  clientTypeId: string;
};
