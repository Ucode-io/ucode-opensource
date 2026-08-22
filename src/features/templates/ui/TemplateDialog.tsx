import { useTranslation } from "react-i18next";
import { Modal } from "@/shared/ui/modal";
import { useApplyTemplate, useTemplates, type Template } from "../api/templates";

/**
 * Выбор шаблона: готовый набор таблиц, который разворачивается в проект.
 *
 * Одним списком и одним действием. Старая админка показывала здесь ещё
 * и выбор отдельных таблиц, функций и микрофронтендов внутри шаблона —
 * но ручка `execute` читает из тела только `id` и `tables` целиком
 * (template.go:29), то есть выбор ничего не менял.
 */
export function TemplateDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const { templates, isLoading, error } = useTemplates(true);
  const apply = useApplyTemplate();

  return (
    <Modal onClose={onClose}>
      <div className="flex max-h-[70vh] w-[32rem] flex-col rounded-xl border border-border bg-surface shadow-modal">
        <header className="flex h-11 shrink-0 items-center border-b border-border px-3">
          <span className="text-sm font-medium">{t("templates.title")}</span>
        </header>

        <div className="flex min-h-0 flex-col gap-2 overflow-y-auto p-2">
          {error ? (
            <p className="p-4 text-sm text-danger">{t("templates.failed")}</p>
          ) : isLoading ? (
            <p className="p-4 text-sm text-fg-muted">{t("common.loading")}</p>
          ) : !templates.length ? (
            <p className="p-4 text-sm text-fg-muted">{t("templates.empty")}</p>
          ) : (
            templates.map((template: Template) => (
              <button
                key={template.id}
                type="button"
                disabled={apply.isPending}
                onClick={() => apply.mutate(template, { onSuccess: onClose })}
                className="flex items-center gap-3 rounded-lg border border-border p-2 text-left transition-colors hover:bg-surface-hover disabled:opacity-50"
              >
                {template.photo && (
                  <img
                    src={template.photo}
                    alt=""
                    className="size-10 shrink-0 rounded-md object-cover"
                  />
                )}
                <span className="flex min-w-0 flex-col">
                  <span className="truncate text-sm font-medium">{template.name}</span>
                  {template.description && (
                    <span className="truncate text-xs text-fg-muted">{template.description}</span>
                  )}
                </span>
              </button>
            ))
          )}
        </div>
      </div>
    </Modal>
  );
}
