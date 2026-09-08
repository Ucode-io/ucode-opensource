import { useTranslation } from "react-i18next";
import type { TranslationKey } from "@/shared/lib/i18n";
import { Button } from "@/shared/ui/button";
import { Modal } from "@/shared/ui/modal";
import { CREATABLE, RESOURCE_LABELS } from "../../api/resources";
import { ResourceIcon } from "./ResourceIcon";

/**
 * Выбор типа перед созданием.
 *
 * Отдельным шагом, а не списком в форме: поля у типов разные, и форма,
 * которая перестраивается под выбранный пункт списка, — это две формы
 * в одной. Заодно на шаге выбора есть место сказать, зачем каждый тип
 * нужен: «Superset» в выпадающем списке не объясняет ничего.
 *
 * Показываются только те типы, которые ЭТА ручка действительно заводит.
 * Google Drive, Telegram и остальные подключаются своей дверью — шлюз
 * на попытку завести их отсюда отвечает отказом со ссылкой на неё
 * (`project_resource.go:56`).
 */
export function ResourceTypePicker({
  onPick,
  onClose,
}: {
  onPick: (kind: string) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();

  return (
    <Modal onClose={onClose}>
      <div className="flex w-full max-w-2xl flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal">
        <div>
          <h2 className="text-base font-semibold">{t("resources.pickType")}</h2>
          <p className="mt-0.5 text-xs text-fg-subtle">{t("resources.pickTypeHint")}</p>
        </div>

        <div className="flex max-h-[60vh] flex-col gap-4 overflow-y-auto">
          {CREATABLE.map(({ group, kinds }) => (
            <div key={group} className="flex flex-col gap-2">
              <span className="text-2xs font-medium tracking-wide text-fg-subtle uppercase">
                {t(`resources.group.${group}` as TranslationKey)}
              </span>

              <div className="grid grid-cols-2 gap-2">
                {kinds.map((kind) => (
                  <button
                    key={kind}
                    type="button"
                    onClick={() => onPick(kind)}
                    className="flex items-start gap-2.5 rounded-lg border border-border p-3 text-left transition-colors hover:border-border-strong hover:bg-surface-hover"
                  >
                    <span className="mt-0.5 text-fg-muted">
                      <ResourceIcon kind={kind} />
                    </span>

                    <span className="min-w-0">
                      <span className="block text-sm font-medium">{RESOURCE_LABELS[kind]}</span>
                      <span className="mt-0.5 block text-xs text-fg-subtle">
                        {t(`resources.about.${kind}` as TranslationKey)}
                      </span>
                    </span>
                  </button>
                ))}
              </div>
            </div>
          ))}
        </div>

        <div className="flex justify-end">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
