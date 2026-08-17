import { session } from "@/shared/api/session";
import { useSession } from "@/shared/api/use-session";
import type { Permission } from "./types";

/**
 * Права роли на таблицу — то, что рисовать, а не то, что разрешено.
 * Настоящую проверку делает сервер; здесь мы только не показываем
 * человеку кнопку, которая ему всё равно ответит 403.
 *
 * Приезжают в ответе логина и обновления токена, лежат в сессии
 * (shared/api/session) и переживают перезагрузку.
 */

/** Роль, которой права не проверяют вовсе. Имя приходит из бэкенда как есть. */
const SUPER_ROLE = "DEFAULT ADMIN";

/**
 * Всё разрешено. Три случая, и все три — «мы не знаем», а не «нельзя»:
 * права ещё не приехали, роль суперадминская, слага таблицы нет.
 *
 * Молчаливый запрет здесь дороже лишней кнопки: сервер всё равно
 * ответит 403, а панель, исчезнувшая из-за незагруженного ответа,
 * читается как сломанное приложение.
 */
const ALL: Permission = {
  tableSlug: "",
  read: true,
  write: true,
  update: true,
  delete: true,
  settings: true,
  columns: true,
  fixColumn: true,
  excelMenu: true,
  viewCreate: true,
  addField: true,
  group: true,
  tabGroup: true,
};

export function storePermissions(permissions: Permission[]) {
  const map = Object.fromEntries(
    permissions
      .filter((permission) => permission.tableSlug)
      .map(({ tableSlug, ...flags }) => [tableSlug, flags]),
  );

  session.setPermissions(map);
}

/** Права на конкретную таблицу. Без слага и без данных — всё разрешено. */
export function useTablePermission(tableSlug: string | undefined): Permission {
  const store = useSession();
  const map = store.getPermissions();

  if (!tableSlug || store.getProfile()?.role === SUPER_ROLE) return ALL;

  const flags = map[tableSlug];
  // Ответ ещё не приехал (первый рендер до refresh) — не запрещаем.
  if (!flags) return ALL;

  return { ...ALL, ...flags, tableSlug };
}
