import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useDataLanguages } from "@/features/workspace";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import { keys } from "@/shared/lib/query-keys";
import { menuUpdateBody } from "./mutations";
import { SYSTEM_MENUS, isSystemMenu } from "../model/system-menus";
import type { MenuNode } from "../model/types";
import type { MenuDto, MenusResponseDto } from "./dto";
import { toMenuNode } from "./normalize";

/**
 * Корень дерева меню. UUID захардкожен в SQL бэкенда
 * (ucode_go_object_builder_service/storage/postgres/menu.go:699) и одинаков
 * для всех проектов, поэтому он константа, а не переменная окружения.
 *
 * Запрос без parent_id возвращает не список, а сам этот корневой пункт —
 * поэтому передавать parent_id обязательно всегда.
 */
export const ROOT_MENU_ID = SYSTEM_MENUS.ROOT;

/**
 * Дети одного уровня. Бэкенд не отдаёт дерево целиком: каждая папка —
 * отдельный запрос, и он уходит только когда папку раскрыли.
 */
export function useMenuChildren(parentId: string, enabled = true) {
  /*
   * Язык ДАННЫХ, а не локаль интерфейса: подпись пункта лежит
   * в attributes.label_<код языка проекта> — тех же кодах, что у полей
   * и view. С локалью ru/en/uz эти ключи совпадают только случайно,
   * и у проекта с языками en+cyr сайдбар показывал базовые подписи.
   */
  const { current: language } = useDataLanguages();
  const session = useSession();
  const projectId = session.getProjectId() ?? "";
  const envId = session.getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: keys.menus.children(projectId, envId, parentId),
    queryFn: () =>
      api.get<MenusResponseDto>("/v3/menus", { params: { parent_id: parentId } }),
    enabled: enabled && Boolean(projectId) && Boolean(parentId),
    // Меню меняет админ, а не пользователь — держим дольше общего правила.
    staleTime: 5 * 60_000,
    select: (data) => toNodes(data.menus ?? [], language),
  });

  return { items: query.data ?? [], isLoading: query.isLoading, error: query.error };
}

export function toNodes(menus: MenuDto[], language: string): MenuNode[] {
  // Порядок ответа сохраняем как есть: сервер уже отсортировал по "order",
  // а само поле до клиента не доезжает — сортировать нечем и незачем.
  return menus
    .filter((dto) => dto.id)
    .map((dto, index) => toMenuNode(dto, language, index))
    // Без права чтения пункт не показывается вовсе — так же, как в старой
    // версии. Показать пункт, который всё равно вернёт отказ, хуже, чем
    // не показать: человек будет думать, что сломалось.
    .filter((node) => node.can.read)
    // Системные пункты (Настройки, Файлы, Пользователи) живут на своих
    // экранах, а не в дереве меню.
    .filter((node) => !isSystemMenu(node.id));
}

/** Один пункт по id — для экрана. Дерево загружено не целиком, искать в нём нечего. */
export function useMenu(menuId: string) {
  const { current: language } = useDataLanguages();
  const session = useSession();
  const projectId = session.getProjectId() ?? "";
  const envId = session.getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: keys.menus.detail(projectId, envId, menuId),
    queryFn: () => api.get<MenuDto>(`/v3/menus/${menuId}`),
    enabled: Boolean(projectId) && Boolean(menuId),
    staleTime: 5 * 60_000,
    select: (dto) => toMenuNode(dto, language),
  });

  return query.data;
}

type Move = {
  parentId: string;
  /** Сам перетаскиваемый пункт: PUT перезаписывает строку целиком. */
  node: MenuNode;
  /** Уровень, откуда тащили. Нужен, чтобы убрать пункт из его кэша. */
  fromParentId: string;
  /** Порядок соседей после перетаскивания — индекс задаёт order. */
  siblings: MenuNode[];
  /** Заполняется, только если пункт сменил папку. */
  movedId?: string;
};

/** Переставляет сырые пункты уровня в порядке, который задал пользователь. */
export function applyOrder(
  cached: MenusResponseDto | undefined,
  order: MenuNode[],
  moved: MenuDto | undefined,
): MenusResponseDto {
  const byId = new Map((cached?.menus ?? []).map((dto) => [dto.id, dto]));
  if (moved?.id) byId.set(moved.id, moved);

  const menus = order.map((node) => byId.get(node.id)).filter((dto): dto is MenuDto => Boolean(dto));

  return { ...cached, menus, count: menus.length };
}

/** Убирает пункт с уровня, откуда его утащили в другую папку. */
export function removeFromLevel(
  cached: MenusResponseDto | undefined,
  movedId: string,
): MenusResponseDto {
  const menus = (cached?.menus ?? []).filter((dto) => dto.id !== movedId);
  return { ...cached, menus, count: menus.length };
}

/**
 * Сохранение порядка. Бэкенд пишет `order = индекс + 1`
 * (storage/postgres/menu.go:974), поэтому поле order в теле не нужно —
 * значение имеет только позиция в массиве.
 */
export function useReorderMenus() {
  const queryClient = useQueryClient();
  const session = useSession();
  const projectId = session.getProjectId() ?? "";
  const envId = session.getEnvironmentId() ?? "";

  return useMutation({
    mutationFn: async ({ parentId, siblings, movedId, node }: Move) => {
      // Сначала смена родителя: иначе порядок применится к старому списку.
      if (movedId) {
        await api.put("/v3/menus", menuUpdateBody(node, { parentId }, projectId));
      }

      await api.put("/v3/menus/menu-order", {
        menus: siblings.map((node) => ({ id: node.id })),
        project_id: projectId,
      });
    },
    // Инвалидируем все уровни: пункт мог переехать между папками.
    /**
     * Порядок применяется в кэше сразу, до ответа сервера. Иначе пункт
     * после броска отскакивает на старое место и возвращается обратно
     * только после перезапроса — выглядит как сбой перетаскивания.
     */
    onMutate: async ({ parentId, fromParentId, siblings, movedId }: Move) => {
      // Иначе ответ уже летящего запроса перезапишет нашу перестановку.
      await queryClient.cancelQueries({ queryKey: keys.menus.all });

      const targetKey = keys.menus.children(projectId, envId, parentId);
      const sourceKey = keys.menus.children(projectId, envId, fromParentId);

      const snapshot = {
        target: queryClient.getQueryData<MenusResponseDto>(targetKey),
        source: queryClient.getQueryData<MenusResponseDto>(sourceKey),
        targetKey,
        sourceKey,
      };

      // Перенос между папками: сырой пункт лежит в кэше исходного уровня.
      const movedDto = movedId
        ? (snapshot.source?.menus ?? []).find((dto) => dto.id === movedId)
        : undefined;

      queryClient.setQueryData(targetKey, applyOrder(snapshot.target, siblings, movedDto));

      if (movedId && parentId !== fromParentId) {
        queryClient.setQueryData(sourceKey, removeFromLevel(snapshot.source, movedId));
      }

      return snapshot;
    },

    // Сервер не принял — возвращаем ровно то, что было.
    onError: (_error, _vars, snapshot) => {
      if (!snapshot) return;
      queryClient.setQueryData(snapshot.targetKey, snapshot.target);
      queryClient.setQueryData(snapshot.sourceKey, snapshot.source);
    },

    onSettled: () => queryClient.invalidateQueries({ queryKey: keys.menus.all }),
  });
}
