import { useTranslation } from "react-i18next";
import type { TranslationKey } from "@/shared/lib/i18n";
import { Checkbox } from "@/shared/ui/checkbox";
import { Field, Input } from "@/shared/ui/input";
import { PasswordInput } from "@/shared/ui/password-input";
import { RESOURCE_SPECS } from "../../api/resources";

/**
 * Поля настроек одного типа ресурса.
 *
 * Набор полей — это данные (`RESOURCE_SPECS`), а не разметка: в старой
 * админке на каждый тип была своя форма, и пять из них отличались только
 * подписями. Здесь тип говорит, какие у него поля, а рисуются они
 * одинаково.
 *
 * Подпись поля ищется по его же имени (`resources.field.<имя>`): второго
 * имени у поля нет, и таблицы соответствий, которая может разъехаться,
 * тоже нет.
 */
export function ResourceFields({
  kind,
  values,
  readOnly,
  creating = false,
  onChange,
}: {
  kind: string;
  values: Record<string, string>;
  /** Учётные данные выданы бэкендом: показать можно, править нечем. */
  readOnly: boolean;
  /**
   * Форма заведения. У баз набор там шире: спрашивается пароль и имя
   * подключения, которых потом в списке не видно (см. createFields).
   */
  creating?: boolean;
  onChange: (values: Record<string, string>) => void;
}) {
  const { t } = useTranslation();
  const spec = RESOURCE_SPECS[kind];
  const fields = (creating && spec?.createFields) || spec?.fields || [];

  if (!fields.length) return null;

  return (
    <div className="grid grid-cols-2 gap-3">
      {fields.map((field) => {
        const value = values[field.key] ?? "";
        const set = (next: string) => onChange({ ...values, [field.key]: next });

        /* Флажок — своей строкой, а не в колодце Field: подпись у него
           справа от квадрата, и сверху ей стоять не над чем. */
        if (field.kind === "toggle") {
          return (
            <label
              key={field.key}
              className="col-span-2 flex h-9 cursor-pointer items-center gap-2 self-end"
            >
              <Checkbox
                checked={value === "true"}
                disabled={readOnly}
                onChange={(event) => set(String(event.target.checked))}
              />
              <span className="text-xs font-medium text-fg-muted">
                {t(`resources.field.${field.key}` as TranslationKey)}
              </span>
            </label>
          );
        }

        return (
          <Field
            key={field.key}
            label={t(`resources.field.${field.key}` as TranslationKey)}
            {...(field.hint ? { hint: t(`resources.fieldHint.${field.key}` as TranslationKey) } : {})}
          >
            {field.kind === "password" ? (
              /* Пароль выданного бэкендом ресурса именно смотрят — глазок
                 нужен и в режиме чтения. */
              <PasswordInput
                value={value}
                readOnly={readOnly}
                onChange={(event) => set(event.target.value)}
              />
            ) : (
              <Input
                type={field.kind === "number" ? "number" : "text"}
                min={field.kind === "number" ? 0 : undefined}
                value={value}
                readOnly={readOnly}
                onChange={(event) => set(event.target.value)}
              />
            )}
          </Field>
        );
      })}
    </div>
  );
}
