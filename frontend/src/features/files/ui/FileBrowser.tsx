import { useEffect, useRef, useState, type ChangeEvent } from "react";
import { IconFile, IconSearch, IconTrash, IconUpload } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { fileUrl, useUploadFiles } from "@/features/item";
import { Icon } from "@/shared/ui/icon";
import { Input } from "@/shared/ui/input";
import { Tooltip } from "@/shared/ui/tooltip";
import { useDeleteFiles, useFiles, useRefreshFiles, type StoredFile } from "../api/files";

/**
 * Файловое хранилище проекта: пункт меню типа `MINIO_FOLDER`.
 *
 * Папка одна и задана настройкой пункта (`attributes.path`) — вложенных
 * папок в ручках нет вовсе, поэтому нет и дерева: экран показывает
 * содержимое своей папки, а соседняя папка — это соседний пункт меню.
 *
 * Загрузка идёт тем же загрузчиком, что и файл в ячейку (features/item):
 * ручка одна, и второй её обёртки быть не должно.
 */

/** Картинки показываем самими собой, остальное — значком. */
const IMAGES = new Set(["PNG", "JPG", "JPEG", "GIF", "WEBP", "SVG", "AVIF", "BMP"]);

const MB = 1024 * 1024;

export function FileBrowser({ folder, canWrite }: { folder: string; canWrite: boolean }) {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<Set<string>>(() => new Set());

  const { page, isLoading, hasMore, loadingMore, loadMore, error } = useFiles({ folder, search });
  const remove = useDeleteFiles(folder);
  const refresh = useRefreshFiles(folder);
  const upload = useUploadFiles();

  const input = useRef<HTMLInputElement>(null);
  const sentinel = useRef<HTMLDivElement>(null);

  /*
   * Догрузка по видимости хвоста, а не по обработчику прокрутки: экран
   * лежит в своей области с прокруткой, и слушать окно тут бесполезно.
   */
  useEffect(() => {
    const node = sentinel.current;
    if (!node || !hasMore) return;

    const observer = new IntersectionObserver(
      ([entry]) => entry?.isIntersecting && loadMore(),
      { rootMargin: "200px" },
    );

    observer.observe(node);
    return () => observer.disconnect();
  });

  const pick = (event: ChangeEvent<HTMLInputElement>) => {
    const files = [...(event.target.files ?? [])];
    // Значение сбрасываем сразу: иначе тот же файл второй раз не выберется.
    event.target.value = "";
    if (!files.length) return;

    upload.mutate({ files, folder }, { onSuccess: () => void refresh() });
  };

  const toggle = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (!next.delete(id)) next.add(id);
      return next;
    });

  const chosen = page.files.filter((file) => selected.has(file.id));

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex h-11 shrink-0 items-center gap-2 border-b border-border px-3">
        <span className="text-sm text-fg-muted">
          {t("files.count", { count: page.count })}
        </span>

        <div className="ml-auto flex items-center gap-2">
          <label className="relative">
            <Icon
              as={IconSearch}
              size={14}
              className="pointer-events-none absolute top-1/2 left-2 -translate-y-1/2 text-fg-subtle"
            />
            <Input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t("files.search")}
              aria-label={t("files.search")}
              className="h-7 w-48 pl-7 text-sm"
            />
          </label>

          {chosen.length > 0 && canWrite && (
            <button
              type="button"
              onClick={() =>
                remove.mutate(chosen, { onSuccess: () => setSelected(new Set()) })
              }
              disabled={remove.isPending}
              className="flex h-7 items-center gap-1.5 rounded-md border border-border px-2 text-sm text-danger transition-colors hover:bg-danger-subtle disabled:opacity-50"
            >
              <Icon as={IconTrash} size={14} />
              {t("files.deleteSelected", { count: chosen.length })}
            </button>
          )}

          {canWrite && (
            <>
              <input ref={input} type="file" multiple hidden onChange={pick} />
              <button
                type="button"
                onClick={() => input.current?.click()}
                disabled={upload.isPending}
                className="flex h-7 items-center gap-1.5 rounded-md bg-accent-solid px-3 text-sm font-medium text-accent-fg transition-opacity hover:opacity-90 disabled:opacity-50"
              >
                <Icon as={IconUpload} size={14} />
                {upload.isPending ? t("files.uploading") : t("files.upload")}
              </button>
            </>
          )}
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-3">
        {error ? (
          <p className="p-6 text-sm text-danger">{String(error)}</p>
        ) : isLoading ? (
          <p className="p-6 text-sm text-fg-muted">{t("common.loading")}</p>
        ) : !page.files.length ? (
          <p className="p-6 text-sm text-fg-muted">
            {search ? t("files.nothingFound") : t("files.empty")}
          </p>
        ) : (
          <div className="grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-3">
            {page.files.map((file) => (
              <FileCard
                key={file.id}
                file={file}
                selected={selected.has(file.id)}
                onSelect={() => toggle(file.id)}
              />
            ))}
          </div>
        )}

        <div ref={sentinel} className="h-px" />
        {loadingMore && <p className="p-3 text-xs text-fg-subtle">{t("common.loading")}</p>}
      </div>
    </div>
  );
}

/**
 * Карточка файла: превью, имя, тип и размер.
 *
 * Открывается сам файл в новой вкладке — своей страницы у файла нет:
 * в хранилище смотрят, чтобы взять ссылку или посмотреть картинку,
 * а не чтобы читать про неё карточку.
 */
function FileCard({
  file,
  selected,
  onSelect,
}: {
  file: StoredFile;
  selected: boolean;
  onSelect: () => void;
}) {
  const url = fileUrl(file.link);
  const size = file.size / MB;

  return (
    <div
      className={`group/card relative flex flex-col overflow-hidden rounded-lg border transition-colors ${
        selected ? "border-accent bg-accent-subtle" : "border-border bg-surface hover:bg-surface-hover"
      }`}
    >
      {/* Флажок поверх превью: он нужен реже, чем сам файл, и не должен
          занимать место в карточке. */}
      <button
        type="button"
        onClick={onSelect}
        aria-pressed={selected}
        className={`absolute top-1.5 left-1.5 z-10 size-4 rounded border transition-opacity ${
          selected
            ? "border-accent-solid bg-accent-solid"
            : "border-border-strong bg-surface opacity-0 group-hover/card:opacity-100"
        }`}
      />

      <a
        href={url}
        target="_blank"
        rel="noreferrer"
        className="grid h-24 place-items-center bg-bg"
      >
        {IMAGES.has(file.extension) ? (
          <img src={url} alt={file.title} loading="lazy" className="h-full w-full object-cover" />
        ) : (
          <Icon as={IconFile} size={24} className="text-fg-subtle" />
        )}
      </a>

      <div className="flex min-w-0 flex-col gap-0.5 p-2">
        <Tooltip label={file.title}>
          <span className="truncate text-xs font-medium">{file.title}</span>
        </Tooltip>
        <span className="text-2xs text-fg-subtle">
          {file.extension || "—"} · {size < 0.1 ? size.toFixed(2) : Math.round(size * 10) / 10} MB
        </span>
      </div>
    </div>
  );
}
