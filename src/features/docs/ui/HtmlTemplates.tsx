import { useState } from "react";
import { IconCode, IconLoader2, IconPencil, IconPlus, IconTrash } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Icon } from "@/shared/ui/icon";
import { Modal } from "@/shared/ui/modal";
import {
  useCreateHtmlTemplate,
  useDeleteHtmlTemplate,
  useHtmlTemplates,
  useUpdateHtmlTemplate,
  type HtmlTemplate,
} from "../api/html-templates";

/**
 * HTML-шаблоны таблицы в общем списке печатных форм.
 *
 * Отдельным компонентом, а не строками внутри `DocTemplates`: у них
 * своя ручка, своё хранилище и своя правка — разметкой вместо файла
 * (см. api/html-templates). Общий у двух видов только список, в котором
 * их выбирают.
 */
export function HtmlTemplateRows({ tableSlug }: { tableSlug: string }) {
  const { t } = useTranslation();
  const { templates } = useHtmlTemplates(tableSlug);
  const remove = useDeleteHtmlTemplate(tableSlug);

  const [editing, setEditing] = useState<HtmlTemplate | null>(null);
  const [removing, setRemoving] = useState<HtmlTemplate | null>(null);

  return (
    <>
      {templates.map((template) => (
        <div
          key={template.id}
          className="flex h-8 items-center gap-2 rounded-md px-2 hover:bg-surface-hover"
        >
          <Icon as={IconCode} size={14} className="shrink-0 text-fg-subtle" />

          <span className="min-w-0 flex-1 truncate text-sm text-fg">
            {template.title || t("docs.untitled")}
          </span>

          <button
            type="button"
            onClick={() => setEditing(template)}
            aria-label={t("action.edit")}
            title={t("action.edit")}
            className="grid size-6 shrink-0 place-items-center rounded text-fg-subtle transition-colors hover:bg-surface-active hover:text-fg"
          >
            <Icon as={IconPencil} size={14} />
          </button>
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

      {editing && (
        <HtmlEditor
          tableSlug={tableSlug}
          template={editing}
          onClose={() => setEditing(null)}
        />
      )}

      {removing && (
        <ConfirmDialog
          title={t("docs.deleteTitle", { name: removing.title || t("docs.untitled") })}
          description={t("docs.htmlDeleteDescription")}
          confirmLabel={t("action.delete")}
          busy={remove.isPending}
          onClose={() => setRemoving(null)}
          onConfirm={() => remove.mutate(removing, { onSuccess: () => setRemoving(null) })}
        />
      )}
    </>
  );
}

/**
 * Завести HTML-шаблон. Он создаётся пустым и сразу открывается
 * на правку: разметку всё равно писать, а лишний шаг «назовите, потом
 * откройте» ничего не добавляет.
 */
export function NewHtmlTemplateButton({ tableSlug }: { tableSlug: string }) {
  const { t } = useTranslation();
  const create = useCreateHtmlTemplate(tableSlug);
  const [created, setCreated] = useState<HtmlTemplate | null>(null);

  return (
    <>
      <button
        type="button"
        disabled={create.isPending}
        /* Ответ ручки — сама заведённая запись, и открываем мы её,
           а не ищем потом в списке. */
        onClick={() => create.mutate(t("docs.htmlNew"), { onSuccess: setCreated })}
        className="flex h-8 items-center justify-center gap-1.5 rounded-md border border-border-strong text-sm text-fg transition-colors hover:bg-surface-hover disabled:opacity-40"
      >
        <Icon as={create.isPending ? IconLoader2 : IconPlus} size={14} className={create.isPending ? "animate-spin" : ""} />
        {t("docs.htmlCreate")}
      </button>

      {/* Заведённый без id открывать нечем: правка ходит по нему. */}
      {created?.id && (
        <HtmlEditor
          tableSlug={tableSlug}
          template={created}
          onClose={() => setCreated(null)}
        />
      )}
    </>
  );
}

/**
 * Правка шаблона: имя и разметка.
 *
 * Разметка правится исходником, в обычном поле. Редактора с кнопками
 * здесь нет намеренно — причина та же, по которой мы не встраиваем
 * ONLYOFFICE ради docx: это отдельный тяжёлый редактор ради поля,
 * в котором пишут `{slug}` (см. api/html-templates).
 */
function HtmlEditor({
  tableSlug,
  template,
  onClose,
}: {
  tableSlug: string;
  template: HtmlTemplate;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const update = useUpdateHtmlTemplate(tableSlug);
  const [draft, setDraft] = useState(template);

  return (
    <Modal onClose={onClose}>
      <form
        className="flex h-[80vh] w-full max-w-3xl flex-col gap-3 rounded-xl border border-border bg-surface p-5 shadow-modal"
        onSubmit={(event) => {
          event.preventDefault();
          update.mutate(draft, { onSuccess: onClose });
        }}
      >
        <input
          value={draft.title}
          onChange={(event) => setDraft({ ...draft, title: event.target.value })}
          aria-label={t("docs.htmlName")}
          placeholder={t("docs.htmlName")}
          className="h-8 rounded-md border border-border-strong bg-surface px-2 text-sm text-fg outline-none focus:border-accent"
        />

        <textarea
          value={draft.html}
          onChange={(event) => setDraft({ ...draft, html: event.target.value })}
          aria-label={t("docs.htmlSource")}
          spellCheck={false}
          className="min-h-0 flex-1 resize-none rounded-md border border-border-strong bg-surface p-2 font-mono text-xs text-fg outline-none focus:border-accent"
        />

        <p className="text-2xs text-fg-subtle">{t("docs.variablesHint")}</p>

        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button type="submit" disabled={update.isPending}>
            {update.isPending ? t("common.saving") : t("action.save")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
