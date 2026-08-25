import { useTranslation } from "react-i18next";
import type { TranslationKey } from "@/shared/lib/i18n";
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
  onChange,
}: {
  kind: string;
  values: Record<string, string>;
  /** Учётные данные выданы бэкендом: показать можно, править нечем. */
  readOnly: boolean;
  onChange: (values: Record<string, string>) => void;
}) {
  const { t } = useTranslation();
  const fields = RESOURCE_SPECS[kind]?.fields ?? [];

  if (!fields.length) return null;

  return (
    <div className="grid grid-cols-2 gap-3">
      {fields.map((field) => {
        const value = values[field.key] ?? "";
        const set = (next: string) => onChange({ ...values, [field.key]: next });

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
