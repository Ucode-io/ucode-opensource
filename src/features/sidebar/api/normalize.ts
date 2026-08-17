import { kindOf, type MenuNode } from "../model/types";
import type { MenuDto } from "./dto";

/**
 * Один сырой пункт → доменный. Дерева здесь нет: бэкенд отдаёт меню
 * по одному уровню за запрос, и уровни склеиваются в компоненте
 * по мере раскрытия папок.
 */

/**
 * Подписи по языкам ДАННЫХ: attributes.label_<код>.
 *
 * Язык здесь тот же, что у полей и view, — из набора проекта, а не
 * локаль интерфейса (см. CONTEXT, Data Language). Старая админка
 * писала эти ключи, насильно переключив i18n на первый язык проекта,
 * и потому у неё label_<локаль> случайно совпадал с label_<язык данных>.
 */
const LABEL_PREFIX = "label_";

export function pickLabels(attributes: Record<string, unknown> | undefined): Record<string, string> {
  const labels: Record<string, string> = {};

  for (const [key, value] of Object.entries(attributes ?? {})) {
    if (!key.startsWith(LABEL_PREFIX) || typeof value !== "string" || !value.trim()) continue;
    labels[key.slice(LABEL_PREFIX.length)] = value;
  }

  return labels;
}

/** Подпись на языке данных, иначе базовая колонка label. */
function pickLabel(dto: MenuDto, language: string): string {
  return pickLabels(dto.attributes)[language]?.trim() || dto.label?.trim() || "—";
}

/**
 * Адрес ссылки берётся из данных, поэтому пропускаем только http и https.
 *
 * Это не перестраховка: пункт меню рисуется как <a href={…}>, и адрес
 * вида javascript:... выполнил бы чужой код по клику. Значение приходит
 * из поля, которое заполняет пользователь, — то есть это открытый ввод.
 */
export function safeHref(value: unknown): string | undefined {
  if (typeof value !== "string" || !value.trim()) return undefined;

  try {
    // Без базового адреса: ссылка в меню обязана быть абсолютной.
    // Относительный путь — это внутренний переход, а не внешняя ссылка,
    // и вести себя он должен иначе.
    const url = new URL(value.trim());
    return url.protocol === "http:" || url.protocol === "https:" ? value : undefined;
  } catch {
    return undefined;
  }
}

function pickHref(dto: MenuDto): string | undefined {
  for (const key of ["website_link", "link"]) {
    const href = safeHref(dto.attributes?.[key]);
    if (href) return href;
  }
  return undefined;
}

function pickPermissions(dto: MenuDto) {
  const p = dto.data?.permission ?? {};

  return {
    read: p["read"] ?? false,
    // Создание внутри папки идёт по write: колонки "create" в таблице
    // menu_permission нет, и бэкенд её не отдаёт (menu.go, permission map).
    // Пока это поле читалось, «Создать таблицу» не показывалось никогда.
    write: p["write"] ?? false,
    update: p["update"] ?? false,
    delete: p["delete"] ?? false,
    // Бэкенд называет это menu_settings; наружу отдаём одним словом.
    settings: p["menu_settings"] ?? false,
  };
}

/**
 * Порядок задаёт сервер: SQL сортирует по `order` (menu.go, ORDER BY),
 * но самого поля `order` в ответе нет — его нет в MenuForGetAll.
 * Поэтому позиция берётся из индекса в ответе, а клиент НИЧЕГО не
 * пересортировывает: любая своя сортировка здесь ломает серверную.
 */
export function toMenuNode(dto: MenuDto, language: string, index = 0): MenuNode {
  const type = dto.type ?? "";
  const href = pickHref(dto);

  return {
    id: dto.id ?? "",
    label: pickLabel(dto, language),
    labels: pickLabels(dto.attributes),
    icon: dto.icon ?? "",
    type,
    kind: kindOf(type),
    order: index,
    isStatic: dto.is_static ?? false,
    parentId: dto.parent_id || null,
    can: pickPermissions(dto),
    children: [],
    raw: dto,
    ...(href ? { href } : {}),
  };
}
