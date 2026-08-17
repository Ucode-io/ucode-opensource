import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import { keys } from "@/shared/lib/query-keys";
import type { MenuNode } from "../model/types";

/** Что можно задать при создании и изменении пункта меню. */
export type MenuInput = {
  /**
   * Подписи по языкам ДАННЫХ. Пишутся и ключами attributes.label_<код>,
   * и базовой колонкой label: колонка одна на все языки, и пустой она
   * быть не должна — по ней пункт находят там, где языка данных нет.
   */
  labels: Record<string, string>;
  icon: string;
  type: string;
  parentId: string;
  /** Только у TABLE: имя таблицы в базе. */
  slug?: string;
  /** Свободный мешок бэкенда: адрес ссылки и подписи по языкам. */
  attributes?: Record<string, unknown>;
};

/**
 * Изменения меню. Все три мутации инвалидируют весь ключ menus: пункт
 * мог переехать между уровнями, и точечная инвалидация одного уровня
 * оставила бы второй устаревшим.
 */
function useMenuMutation<TVars>(run: (vars: TVars, projectId: string) => Promise<unknown>) {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: (vars: TVars) => run(vars, projectId),
    onSettled: () => queryClient.invalidateQueries({ queryKey: keys.menus.all }),
  });
}

/**
 * Таблица создаётся не через /v3/menus.
 *
 * POST /v3/menus с type=TABLE вставляет пустой пункт и тут же идёт искать
 * слаг по table_id, которого нет: «failed to get table slug: invalid input
 * syntax for type uuid». Таблицы нет — брать слаг неоткуда.
 *
 * Настоящая ручка — POST /v1/table: она в одной транзакции заводит саму
 * таблицу, поле guid, пункт меню (menu_id = родитель), layout, view
 * и права всех ролей (object_builder/storage/postgres/table.go).
 */
export function useCreateMenu() {
  return useMenuMutation<MenuInput>((input, projectId) => {
    if (input.type === "TABLE") {
      return api.post("/v1/table", {
        label: baseLabel(input.labels),
        slug: input.slug,
        icon: input.icon,
        show_in_menu: true,
        menu_id: input.parentId,
        // Обязательно, даже пустой. Пункт меню для таблицы бэкенд пишет
        // этим значением как есть (table.go:140), и без него в колонку
        // menu.attributes ложится NULL вместо '{}'. После этого весь
        // уровень перестаёт читаться: GetAll разбирает attributes без
        // проверки и падает с "unexpected end of JSON input".
        //
        // Этим же мешком задаётся имя таблицы на языках данных: бэкенд
        // кладёт его и таблице, и её пункту меню.
        attributes: labelAttributes(input.labels),
      });
    }

    return api.post("/v3/menus", {
      label: baseLabel(input.labels),
      icon: input.icon,
      type: input.type,
      parent_id: input.parentId,
      project_id: projectId,
      attributes: { ...labelAttributes(input.labels), ...input.attributes },
    });
  });
}

/** Что меняем у существующего пункта. Остальное берётся из него самого. */
export type MenuPatch = {
  /** Подписи по языкам данных целиком. Не заданы — имя не трогаем. */
  labels?: Record<string, string>;
  icon?: string;
  parentId?: string;
  attributes?: Record<string, unknown>;
};

/**
 * Тело PUT /v3/menus.
 *
 * Запрос не частичный: SQL безусловно пишет label, parent_id, layout_id,
 * table_id, type, icon и attributes (storage/postgres/menu.go:943). Поле,
 * которого нет в теле, обнуляется — переименование таблицы стёрло бы ей
 * table_id, — а пустой type бэкенд отвергает («unsupported menu type»).
 * Поэтому тело всегда собирается из пункта целиком.
 */
export function menuUpdateBody(node: MenuNode, patch: MenuPatch, projectId: string) {
  const raw = node.raw;

  return {
    id: node.id,
    // raw.label, а не node.label: в узле лежит подпись для показа, она
    // могла прийти из attributes.label_<язык данных>.
    label: (patch.labels && baseLabel(patch.labels)) || raw.label || "",
    icon: patch.icon ?? raw.icon ?? "",
    type: raw.type ?? node.type,
    parent_id: patch.parentId ?? raw.parent_id ?? "",
    table_id: raw.table_id ?? "",
    layout_id: raw.layout_id ?? "",
    attributes: {
      ...raw.attributes,
      ...(patch.labels ? labelAttributes(patch.labels) : {}),
      ...patch.attributes,
    },
    project_id: projectId,
  };
}

export function useUpdateMenu() {
  return useMenuMutation<{ node: MenuNode } & MenuPatch>(({ node, ...patch }, projectId) =>
    api.put("/v3/menus", menuUpdateBody(node, patch, projectId)),
  );
}

export function useDeleteMenu() {
  return useMenuMutation<{ id: string }>(({ id }, projectId) =>
    api.delete(`/v3/menus/${id}`, { params: { "project-id": projectId } }),
  );
}

/** Подписи по языкам → ключи attributes. Пустые уезжают тоже: так стирают. */
function labelAttributes(labels: Record<string, string>): Record<string, string> {
  return Object.fromEntries(
    Object.entries(labels).map(([code, value]) => [`label_${code}`, value.trim()]),
  );
}

/**
 * Базовая подпись — первое непустое имя. Колонка `label` одна на все
 * языки, и пустой она быть не может: по ней пункт видно в местах,
 * где языка данных нет вовсе.
 */
function baseLabel(labels: Record<string, string>): string {
  for (const value of Object.values(labels)) {
    if (value.trim()) return value.trim();
  }
  return "";
}
