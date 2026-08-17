import { kindOf, type MenuNode } from "../model/types";
import type { MenuDto } from "./dto";

/**
 * Один сырой пункт → доменный. Дерева здесь нет: бэкенд отдаёт меню
 * по одному уровню за запрос, и уровни склеиваются в компоненте
 * по мере раскрытия папок.
 */

/**
 * Подпись берётся из attributes.label_<язык>, иначе из label.
 * Ключ динамический, поэтому разворачивается здесь, а не в компоненте.
 */
function pickLabel(dto: MenuDto, locale: string): string {
  const localized = dto.attributes?.[`label_${locale}`];
  if (typeof localized === "string" && localized.trim()) return localized;
  return dto.label?.trim() || "—";
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
export function toMenuNode(dto: MenuDto, locale: string, index = 0): MenuNode {
  const type = dto.type ?? "";
  const href = pickHref(dto);

  return {
    id: dto.id ?? "",
    label: pickLabel(dto, locale),
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
