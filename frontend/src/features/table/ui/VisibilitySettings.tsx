import { useTranslation } from "react-i18next";
import { Dropdown } from "@/shared/ui/dropdown";
import { Checkbox } from "@/shared/ui/checkbox";
import { HIDE_COMPARISONS, type FieldDraft } from "../model/field-draft";
import { localized, type Field } from "../model/types";
import { Labeled } from "./FormulaSettings";

/**
 * Условная видимость: показывать поле в карточке, только когда значение
 * СОСЕДНЕГО поля совпало с заданным.
 *
 * Названо тем, что делает, а не тем, как лежит в базе: ключи там
 * называются `hide_path*`, но применяются наоборот — совпало, значит
 * показать (разобрано в `item/model/visibility`).
 *
 * Следить можно за любым полем таблицы, кроме себя: поле, спрятанное
 * по собственному значению, не заполнить.
 */
export function VisibilitySettings({
  draft,
  fields,
  language,
  onChange,
}: {
  draft: FieldDraft;
  /** Поля ЭТОЙ таблицы: из них выбирают то, за которым следим. */
  fields: Field[];
  language: string;
  onChange: (next: Partial<FieldDraft>) => void;
}) {
  const { t } = useTranslation();

  const sources = fields.filter((field) => field.slug && field.slug !== draft.slug);
  if (!sources.length) return null;

  const watched = sources.find((field) => field.slug === draft.hideField);
  const options = [...(watched?.options.values() ?? [])];
  /*
   * У поля с вариантами значение выбирают, а не набирают: слаг варианта
   * человек не знает, а именно он лежит в записи.
   */
  const multi = watched?.type === "MULTISELECT";
  /*
   * Числовое сравнение живёт в ключе `type`, который у поля FORMULA
   * занят видом агрегата. Двух смыслов у одного ключа не бывает,
   * поэтому агрегату выбор не предлагается — остаётся равенство.
   */
  const numeric = watched?.type === "NUMBER" && draft.type !== "FORMULA";

  const pick = (slug: string) =>
    // Значение выбиралось у прежнего поля: у нового такого варианта нет,
    // и оставить его значит сохранить условие, которое не совпадёт никогда.
    onChange({ hideField: slug, hideValues: [], hideMulti: false, hideCompare: "" });

  return (
    <div className="flex flex-col gap-1.5 px-2 py-1">
      <span className="text-2xs text-fg-muted">{t("visibility.title")}</span>

      <Labeled label={t("visibility.field")}>
        <Dropdown
          size="sm"
          value={draft.hideField}
          placeholder="—"
          items={sources.map((field) => ({
            value: field.slug,
            label: localized(field.labels, language, field.label) || field.slug,
          }))}
          onChange={pick}
        />
      </Labeled>

      {draft.hideField && (
        <>
          {numeric && (
            <Labeled label={t("visibility.compare")}>
              <Dropdown
                size="sm"
                value={draft.hideCompare}
                placeholder={t("visibility.compare.equals")}
                items={HIDE_COMPARISONS.map((item) => ({
                  value: item,
                  label: t(`visibility.compare.${item}`),
                }))}
                onChange={(hideCompare) => onChange({ hideCompare })}
              />
            </Labeled>
          )}

          <Labeled label={t("visibility.value")}>
            {options.length ? (
              <div className="flex flex-col">
                {options.map((option) => (
                  <label
                    key={option.value}
                    className="flex h-7 cursor-pointer items-center gap-2 rounded-md px-1 transition-colors hover:bg-surface-hover"
                  >
                    <Checkbox
                      checked={draft.hideValues.includes(option.value)}
                      onChange={(event) =>
                        onChange({
                          hideValues: event.target.checked
                            ? multi
                              ? [...draft.hideValues, option.value]
                              : [option.value]
                            : draft.hideValues.filter((value) => value !== option.value),
                          hideMulti: multi,
                        })
                      }
                    />
                    <span className="min-w-0 flex-1 truncate text-xs text-fg">
                      {localized(option.labels, language, option.label) || option.value}
                    </span>
                  </label>
                ))}
              </div>
            ) : (
              <input
                value={draft.hideValues[0] ?? ""}
                placeholder={t("visibility.valuePlaceholder")}
                onChange={(event) =>
                  onChange({ hideValues: [event.target.value], hideMulti: false })
                }
                className="h-7 w-full rounded-md border border-border-strong bg-surface px-2 text-xs text-fg outline-none focus:border-accent"
              />
            )}
          </Labeled>

          <span className="text-2xs text-fg-subtle">{t("visibility.hint")}</span>
        </>
      )}
    </div>
  );
}
