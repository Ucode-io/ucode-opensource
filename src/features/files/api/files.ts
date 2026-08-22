import { useCallback } from "react";
import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";
import i18n from "@/shared/lib/i18n";

/**
 * Файловое хранилище проекта — то, что в меню лежит пунктом
 * `MINIO_FOLDER`.
 *
 * Ручки те же, что у загрузки файла в ячейку: хранилище одно на проект,
 * а папка — это `attributes.path` пункта меню. Списка папок в ручках
 * нет вовсе: папка задаётся строкой в настройках пункта, и пункт меню
 * ЕСТЬ папка.
 */

/** Сколько файлов за раз. Карточки крупные — экран вмещает около двадцати. */
const PAGE = 20;

export type FileDto = {
  id?: string;
  title?: string;
  /** Путь в хранилище: адрес собирается из него и адреса CDN. */
  link?: string;
  /** Имя объекта в хранилище. Без него файл не удалить. */
  file_name_disk?: string;
  /** Имя, под которым файл скачивается. Из него берётся расширение. */
  file_name_download?: string;
  /** Байты. Приходит и числом, и строкой — модели шлюза расходятся. */
  file_size?: number | string;
};

type FilesResponse = { files?: FileDto[]; count?: number };

export type StoredFile = {
  id: string;
  title: string;
  link: string;
  /** Имя объекта в хранилище — им же файл и удаляется. */
  objectName: string;
  /** Расширение большими буквами: PNG, PDF. Пусто — имени не было. */
  extension: string;
  size: number;
};

export type FilesPage = { files: StoredFile[]; count: number };

const EMPTY: FilesPage = { files: [], count: 0 };

/**
 * Список файлов папки. Прокруткой, а не страницами: хранилище смотрят
 * глазами, и «страница 3» в нём не значит ничего.
 */
export function useFiles({ folder, search }: { folder: string; search: string }) {
  const query = useInfiniteQuery({
    queryKey: keys.files.list(folder, search),
    queryFn: ({ pageParam }) =>
      api.get<FilesResponse>("/v1/files", {
        params: {
          folder_name: folder,
          search,
          limit: PAGE,
          offset: pageParam * PAGE,
        },
      }),
    initialPageParam: 0,
    getNextPageParam: (_last, all) => {
      const loaded = all.reduce((total, page) => total + (page.files?.length ?? 0), 0);
      const count = all[0]?.count ?? 0;
      return loaded < count ? all.length : undefined;
    },
    select: useCallback(
      (data: { pages: FilesResponse[] }): FilesPage => ({
        files: data.pages.flatMap((page) => (page.files ?? []).map(toFile)),
        count: data.pages[0]?.count ?? 0,
      }),
      [],
    ),
  });

  return {
    page: query.data ?? EMPTY,
    isLoading: query.isLoading,
    hasMore: query.hasNextPage && !query.isFetchingNextPage,
    loadingMore: query.isFetchingNextPage,
    loadMore: () => {
      /* `cancelRefetch: false` — второй вызов, пришедший пока летит
         первый, иначе отменяет его и просит ту же порцию заново. */
      if (query.hasNextPage && !query.isFetchingNextPage) {
        void query.fetchNextPage({ cancelRefetch: false });
      }
    },
    error: query.error,
    refetch: () => void query.refetch(),
  };
}

/**
 * Удаление отмеченных файлов.
 *
 * Тело — список пар «id и имя объекта» (`models.FileDeleteRequest`):
 * запись в базе и объект в хранилище удаляются разными частями, и
 * второго имени взять неоткуда, кроме как из списка.
 */
export function useDeleteFiles(folder: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (files: StoredFile[]) =>
      api.delete("/v1/files", {
        data: {
          objects: files.map((file) => ({ object_id: file.id, object_name: file.objectName })),
        },
      }),
    onError: (error) => reportError(error, "files.deleteFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("files.deleted"));
      await queryClient.invalidateQueries({ queryKey: keys.files.folder(folder) });
    },
  });
}

/** Обновить список после загрузки: файлы кладёт общий загрузчик. */
export function useRefreshFiles(folder: string) {
  const queryClient = useQueryClient();

  return () => queryClient.invalidateQueries({ queryKey: keys.files.folder(folder) });
}

export function toFile(dto: FileDto): StoredFile {
  const name = dto.file_name_download?.trim() ?? "";
  const dot = name.lastIndexOf(".");

  return {
    id: dto.id ?? "",
    title: dto.title?.trim() || name || dto.id || "",
    link: dto.link ?? "",
    objectName: dto.file_name_disk ?? "",
    extension: dot > 0 ? name.slice(dot + 1).toUpperCase() : "",
    // Строкой размер приходит от одной ветки шлюза, числом — от другой.
    size: Number(dto.file_size ?? 0) || 0,
  };
}
