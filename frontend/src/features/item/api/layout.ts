import { useMemo } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import { keys } from "@/shared/lib/query-keys";
import { reportError } from "@/shared/lib/toast";
import {
  addSection,
  fieldOrder,
  fieldRights,
  headingSlug,
  hiddenFields,
  moveField,
  removeSection,
  renameSection,
  sections,
  setHeading,
  toggleHidden,
  type Layout,
} from "../model/layout";

/**
 * Порядок полей в drawer и его правка.
 *
 * Раскладка живёт отдельно от view: у пункта меню свой layout, и порядок
 * полей в карточке не трогает колонки таблицы (и наоборот). См.
 * model/layout.ts.
 *
 * Ручка v2, а не v1: v1 отвечает дважды (h.HandleResponse и в ветке
 * успеха, и после неё), и в теле лежат два JSON подряд — разобрать его
 * нельзя.
 */
export function useDrawerLayout({
  tableSlug,
  menuId,
  language,
}: {
  tableSlug: string;
  menuId: string;
  /** Язык ДАННЫХ: на нём читается заголовок мультиязычной карточки. */
  language: string;
}) {
  const queryClient = useQueryClient();
  const session = useSession();
  const envId = session.getEnvironmentId() ?? "";
  const key = keys.layouts.byMenu(envId, tableSlug, menuId);

  const query = useQuery({
    queryKey: key,
    queryFn: () => api.get<Layout>(`/v2/collections/${tableSlug}/layout/${menuId}`),
    enabled: Boolean(tableSlug && menuId),
    // Раскладку меняет админ, а не пользователь при работе со строками.
    staleTime: 5 * 60_000,
  });

  const update = useMutation({
    /*
     * menu_id обязателен, хотя GET его не отдаёт: PUT пишет эту колонку
     * безусловно (layout.go, Update), и тело без него отвязало бы
     * раскладку от пункта меню — следующий GET вернул бы вместо неё
     * общую, помеченную is_default.
     */
    mutationFn: (next: Layout) =>
      api.put<unknown>(`/v2/collections/${tableSlug}/layout`, { ...next, menu_id: menuId }),
    // Новый порядок показывается сразу: иначе поле, брошенное мышью,
    // стоит на старом месте до ответа PUT и перезапроса раскладки.
    onMutate: (next) => {
      const previous = queryClient.getQueryData<Layout>(key);
      queryClient.setQueryData(key, next);
      return previous;
    },
    onError: (error, _next, previous) => {
      if (previous) queryClient.setQueryData(key, previous);
      reportError(error, "common.saveFailed");
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: key }),
  });

  return {
    /** Идёт запись раскладки: панели настроек показывают это спиннером. */
    saving: update.isPending,
    /** Слаги полей в порядке карточки. */
    order: useMemo(() => fieldOrder(query.data), [query.data]),
    /** Слаги полей, спрятанных из карточки (`field_hide_layout`). */
    hidden: useMemo(() => new Set(hiddenFields(query.data)), [query.data]),
    /**
     * Права роли на поля. Приходят с раскладкой, потому что в схеме
     * полей их нет вовсе — см. model/layout, fieldRights.
     */
    rights: useMemo(() => fieldRights(query.data), [query.data]),
    /** Секции карточки: имя и слаги полей. */
    sections: useMemo(() => sections(query.data), [query.data]),
    /** Слаг поля-заголовка карточки. Пусто — заголовка нет. */
    heading: useMemo(() => headingSlug(query.data, language), [query.data, language]),
    /** Поле `moved` встаёт рядом с `target` — до него или после. */
    reorder: (moved: string, target: string, after: boolean) => {
      if (query.data) update.mutate(moveField(query.data, moved, target, after));
    },
    /**
     * Новое поле-заголовок. `variants` — карта «язык → слаг» у мультиязычного
     * поля, иначе null.
     */
    setHeading: (slug: string, variants: Record<string, string> | null) => {
      if (query.data) update.mutate(setHeading(query.data, slug, variants));
    },
    /**
     * Спрятать поле из карточки или вернуть его в неё. Колонкой таблицы
     * оно остаётся: это настройка раскладки, а не поля.
     */
    toggleHidden: (slug: string) => {
      if (query.data) update.mutate(toggleHidden(query.data, slug));
    },
    /** Новая секция в конце карточки. Пустая: поля переносят мышью. */
    addSection: (label: string) => {
      if (query.data) update.mutate(addSection(query.data, label));
    },
    renameSection: (index: number, label: string) => {
      if (query.data) update.mutate(renameSection(query.data, index, label));
    },
    /** Удаление секции. Поля уходят в соседнюю — терять их нельзя. */
    removeSection: (index: number) => {
      if (query.data) update.mutate(removeSection(query.data, index));
    },
  };
}
