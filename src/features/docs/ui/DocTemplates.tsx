import { useRef, useState } from "react";
import { IconCopy, IconFileTypeDocx, IconLoader2, IconTrash } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { useUploadFiles } from "@/features/item";
import type { Field } from "@/features/table";
import { Icon } from "@/shared/ui/icon";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { toast } from "@/shared/lib/toast";
import {
  useCreateDocTemplate,
  useDeleteDocTemplate,
  useDocTemplates,
  templateUrl,
  type DocTemplate,
} from "../api/templates";

/** Куда кладём сам файл шаблона. Отдельно от media: это не данные строк. */
const FOLDER = "docs";

/**
 * Печатные формы таблицы: список шаблонов, загрузка нового и подсказка,
 * какие переменные в нём можно писать.
 *
 * Панель, а не экран. В старой админке «Docs» уводит со своих данных
 * на отдельную страницу, где запись приходится выбирать заново; здесь
 * шаблоны настраивают там же, где остальное про таблицу, а печатают —
 * из самой записи.
 *
 * Переменные показываются из НАШЕЙ схемы, а не ручкой
 * `/v2/docx-template/fields/list`: та отдаёт `{table_id, label, slug}`
 * тех же полей и всегда пустой список связей (docx.go:514 — ключ там
 * к тому же назван «relations:», с двоеточием). Лишний запрос за тем,
 * что уже загружено.
 */
export function DocTemplates({
  tableSlug,
  fields,
}: {
  tableSlug: string;
  /** Поля таблицы: из их слагов собираются переменные шаблона. */
  fields: Field[];
}) {
  const { t } = useTranslation();
  const { templates, isLoading } = useDocTemplates(tableSlug);
  const create = useCreateDocTemplate(tableSlug);
  const remove = useDeleteDocTemplate(tableSlug);
  const upload = useUploadFiles();
  const input = useRef<HTMLInputElement>(null);
  const [removing, setRemoving] = useState<DocTemplate | null>(null);

  const pick = async (file: File | undefined) => {
    if (!file) return;

    const [url] = await upload.mutateAsync({ files: [file], folder: FOLDER });
    if (!url) return;

    /*
     * Имя шаблона — имя файла без расширения. Спрашивать его отдельно
     * значит показать пустое поле над уже выбранным файлом: человек
     * назвал документ, когда сохранял его в Word.
     */
    create.mutate({ title: file.name.replace(/\.docx$/i, ""), fileUrl: url });
  };

  const busy = upload.isPending || create.isPending;

  return (
    <div className="flex max-h-[70vh] flex-col gap-2 overflow-y-auto p-2">
      <div className="flex flex-col gap-0.5">
        {isLoading && <p className="px-1 text-xs text-fg-subtle">{t("common.loading")}</p>}

        {!isLoading && !templates.length && (
          <p className="px-1 text-2xs text-fg-subtle">{t("docs.empty")}</p>
        )}

        {templates.map((template) => (
          <div
            key={template.id}
            className="flex h-8 items-center gap-2 rounded-md px-2 hover:bg-surface-hover"
          >
            <Icon as={IconFileTypeDocx} size={14} className="shrink-0 text-fg-subtle" />
            {/* Имя ведёт на сам файл: шаблон правят в Word, и первым
                делом его скачивают. */}
            <a
              href={templateUrl(template.fileUrl)}
              target="_blank"
              rel="noreferrer"
              className="min-w-0 flex-1 truncate text-sm text-fg hover:underline"
            >
              {template.title || t("docs.untitled")}
            </a>
            <button
              type="button"
              onClick={() => setRemoving(template)}
              aria-label={t("action.delete")}
              className="grid size-6 shrink-0 place-items-center rounded text-fg-subtle transition-colors hover:bg-danger-subtle hover:text-danger"
            >
              <Icon as={IconTrash} size={14} />
            </button>
          </div>
        ))}
      </div>

      <input
        ref={input}
        type="file"
        accept=".docx"
        hidden
        onChange={(event) => {
          void pick(event.target.files?.[0]);
          // Тот же файл во второй раз не даёт события change, если
          // значение не сбросить: перезалить исправленный шаблон
          // под тем же именем — обычное дело.
          event.target.value = "";
        }}
      />

      <button
        type="button"
        disabled={busy}
        onClick={() => input.current?.click()}
        className="flex h-8 items-center justify-center gap-1.5 rounded-md bg-accent text-sm text-accent-fg transition-opacity hover:opacity-90 disabled:opacity-40"
      >
        {busy && <Icon as={IconLoader2} size={14} className="animate-spin" />}
        {t("docs.upload")}
      </button>

      <div className="border-t border-border pt-2">
        <p className="px-1 text-2xs text-fg-muted">{t("docs.variables")}</p>
        <p className="px-1 pb-1 text-2xs text-fg-subtle">{t("docs.variablesHint")}</p>

        <div className="flex flex-col">
          {fields.map((field) => (
            <Variable key={field.id} label={field.label || field.slug} slug={field.slug} />
          ))}
        </div>
      </div>

      {removing && (
        <ConfirmDialog
          title={t("docs.deleteTitle", { name: removing.title || t("docs.untitled") })}
          description={t("docs.deleteDescription")}
          confirmLabel={t("action.delete")}
          busy={remove.isPending}
          onClose={() => setRemoving(null)}
          onConfirm={() =>
            remove.mutate(removing, { onSuccess: () => setRemoving(null) })
          }
        />
      )}
    </div>
  );
}

/**
 * Переменная шаблона: щелчок кладёт `{слаг}` в буфер — писать её
 * человеку предстоит в Word, а не здесь.
 */
function Variable({ label, slug }: { label: string; slug: string }) {
  const { t } = useTranslation();

  return (
    <button
      type="button"
      onClick={() => {
        void navigator.clipboard.writeText(`{${slug}}`);
        toast.success(t("cell.copied"));
      }}
      aria-label={t("cell.copy")}
      title={t("cell.copy")}
      className="flex h-7 items-center gap-2 rounded-md px-2 text-left transition-colors hover:bg-surface-hover"
    >
      <span className="min-w-0 flex-1 truncate text-2xs text-fg-muted">{label}</span>
      <span className="shrink-0 font-mono text-2xs text-fg-subtle">{`{${slug}}`}</span>
      <Icon as={IconCopy} size={12} className="shrink-0 text-fg-subtle" />
    </button>
  );
}
