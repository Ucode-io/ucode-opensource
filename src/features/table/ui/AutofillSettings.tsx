import { useTranslation } from "react-i18next";
import { Checkbox } from "@/shared/ui/checkbox";
import { Dropdown } from "@/shared/ui/dropdown";
import { useTableSchema } from "../api/schema";
import { tableFromSlug, type FieldDraft } from "../model/field-draft";
import { localized, type Relation } from "../model/types";
import { Labeled } from "./FormulaSettings";

/**
 * Автозаполнение: «взять значение из строки, на которую указывает связь».
 *
 * Настройка живёт на поле-приёмнике, а не на связи, и состоит из двух
 * половин: откуда брать (связь + поле чужой таблицы) и когда подставлять.
 *
 * Источником может быть только Many2One НАШЕЙ стороны: подставлять
 * нечего, пока связанная строка не приходит вместе с нашей, а приходит
 * она ровно у таких связей — колонкой `<колонка-связь>_data`
 * (см. Relation.fieldFrom). У таблицы без таких связей блока нет вовсе.
 */
export function AutofillSettings({
  draft,
  relations,
  language,
  onChange,
}: {
  draft: FieldDraft;
  /** Связи ЭТОЙ таблицы. */
  relations: Relation[];
  language: string;
  onChange: (next: Partial<FieldDraft>) => void;
}) {
  const { t } = useTranslation();

  const sources = relations.filter(
    (relation) => relation.type === "Many2One" && relation.toSlug && relation.fieldFrom,
  );

  /*
   * Поля чужой таблицы. Хук сидит выключенным, пока связь не выбрали;
   * колонки не запрашиваем — здесь нужны только имена полей, а не
   * настройки каждой связи чужой таблицы.
   */
  const slug = tableFromSlug(draft.autofillTable);
  const { schema } = useTableSchema(slug || undefined, []);

  if (!sources.length) return null;

  // LOOKUP и LOOKUPS не берём: в такой колонке лежит uuid чужой строки,
  // и подставленный в текстовое поле он ничего не значит.
  const usable = schema.fields.filter(
    (field) => field.type !== "LOOKUP" && field.type !== "LOOKUPS",
  );

  return (
    <div className="flex flex-col gap-1.5 px-2 py-1">
      <span className="text-2xs text-fg-muted">{t("autofill.title")}</span>

      <Labeled label={t("autofill.relation")}>
        <Dropdown
          size="sm"
          value={draft.autofillTable}
          placeholder="—"
          items={sources.map((relation) => ({
            value: `${relation.toSlug}#${relation.fieldFrom}`,
            label: relation.toSlug,
          }))}
          // Поле выбиралось в прежней таблице: в новой такого слага нет,
          // и оставить его значит просить бэкенд о несуществующем поле.
          onChange={(autofillTable) => onChange({ autofillTable, autofillField: "" })}
        />
      </Labeled>

      {slug && (
        <>
          <Labeled label={t("autofill.field")}>
            <Dropdown
              size="sm"
              value={draft.autofillField}
              placeholder="—"
              items={usable.map((field) => ({
                value: field.slug,
                label: localized(field.labels, language, field.label),
              }))}
              onChange={(autofillField) => onChange({ autofillField })}
            />
          </Labeled>

          <label className="flex h-7 cursor-pointer items-center gap-2 rounded-md transition-colors hover:bg-surface-hover">
            <span className="flex-1 truncate text-xs text-fg">{t("autofill.automatic")}</span>
            <Checkbox
              checked={draft.automatic}
              onChange={(event) => onChange({ automatic: event.target.checked })}
            />
          </label>
        </>
      )}
    </div>
  );
}
