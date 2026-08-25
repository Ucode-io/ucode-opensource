import type { AuthSession, Connection, Permission, RecoveryStart } from "../model/types";
import type {
  ConnectionDto,
  ForgotPasswordDto,
  LoginResponseDto,
  PermissionDto,
} from "./dto";

/**
 * Единственное место, где сырые имена бэкенда превращаются в доменные.
 * Правило «одно имя — один источник» из docs/PLAN.md: `a || b || c.d`
 * не пишется в компонентах — вся разноголосица гасится здесь.
 */

/**
 * `'No'` — запрет, всё остальное — разрешение. Именно в эту сторону,
 * а не `=== 'Yes'`: колонок в record_permission два десятка, они
 * добавлялись миграциями по одной, и у старых ролей часть из них
 * пустая. Пустая колонка означает «настройку не трогали», а не «нельзя».
 */
const allowed = (value: string | undefined): boolean => value !== "No";

export function toPermission(dto: PermissionDto): Permission {
  return {
    tableSlug: dto.table_slug ?? "",
    read: allowed(dto.read),
    write: allowed(dto.write),
    update: allowed(dto.update),
    delete: allowed(dto.delete),
    settings: allowed(dto.settings),
    columns: allowed(dto.columns),
    fixColumn: allowed(dto.fix_column),
    excelMenu: allowed(dto.excel_menu),
    viewCreate: allowed(dto.view_create),
    addField: allowed(dto.add_field),
    group: allowed(dto.group),
    tabGroup: allowed(dto.tab_group),
    searchButton: allowed(dto.search_button),
    fieldFilter: allowed(dto.field_filter),
  };
}

export function toSession(dto: LoginResponseDto): AuthSession {
  return {
    userId: dto.user_id ?? "",
    // project_id в ответе логина нет — он приходит в claims токена
    // и в project_data. Проставляется вызывающим кодом.
    projectId: "",
    environmentId: dto.environment_id ?? "",
    roleId: dto.role?.id ?? "",
    clientTypeId: dto.client_type?.id ?? "",
    loginTableSlug: dto.login_table_slug ?? "",
    permissions: (dto.permissions ?? []).map(toPermission),
  };
}

export function toConnection(dto: ConnectionDto): Connection {
  // Подпись записи лежит в поле, имя которого задано view_slug.
  // Это единственное место, где такой динамический ключ разворачивается.
  const labelKey = dto.view_slug ?? "";

  return {
    id: dto.guid ?? "",
    tableSlug: dto.table_slug ?? "",
    options: (dto.options ?? []).map((option) => {
      const id = typeof option["guid"] === "string" ? option["guid"] : "";
      const label = option[labelKey];

      return { id, label: typeof label === "string" && label ? label : id };
    }),
  };
}

/**
 * Ответ первого шага восстановления → что показывать дальше.
 *
 * Три исхода, и все три приезжают со статусом 200: отказа здесь нет
 * вовсе. Логин не нашли — приходит пустой `user_id`; нашли, но почты
 * у пользователя нет — `email_found: false` при непустом `user_id`
 * (handlers/session_v2.go, ForgotPassword).
 */
export function toRecoveryStart(dto: ForgotPasswordDto): RecoveryStart {
  if (!dto.user_id) return { kind: "unknown" };

  return dto.email_found
    ? { kind: "sent", userId: dto.user_id, smsId: dto.sms_id ?? "", email: dto.email ?? "" }
    : { kind: "noEmail", userId: dto.user_id };
}
