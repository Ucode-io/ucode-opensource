import { useRef } from "react";
import { IconPlus, IconTrash } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Checkbox } from "@/shared/ui/checkbox";
import { Chip } from "@/shared/ui/chip";
import { Icon } from "@/shared/ui/icon";
import { Input, Select } from "@/shared/ui/input";
import type { TranslationKey } from "@/shared/lib/i18n";
import { useTableSchema } from "../api/schema";
import {
  AGGREGATES,
  filterParts,
  tableFromSlug,
  type Aggregate,
  type AggregateFilter,
  type FieldDraft,
} from "../model/field-draft";
import { localized, type Field, type Relation } from "../model/types";

/**
 * Настройки полей-формул. Слово одно, поля два, и общего у них ничего:
 *
 *   FORMULA_FRONTEND  выражение по полям ЭТОЙ строки, считает браузер
 *                     на каждый показ ячейки (см. features/item)
 *   FORMULA           «сумма поля X по связанным строкам таблицы Y»,
 *                     считает бэкенд, в строке лежит готовое число
 *
 * Поэтому и экрана два. Общая часть — только место в панели поля.
 */
export function FormulaSettings({
  draft,
  fields,
  relations,
  language,
  onChange,
}: {
  draft: FieldDraft;
  /** Поля ЭТОЙ таблицы: из них собирается выражение. */
  fields: Field[];
  /** Связи ЭТОЙ таблицы: по ним выбирается таблица агрегата. */
  relations: Relation[];
  language: string;
  onChange: (next: Partial<FieldDraft>) => void;
}) {
  if (draft.type === "FORMULA_FRONTEND") {
    return (
      <ExpressionEditor
        formula={draft.formula}
        fields={fields.filter((field) => field.slug !== draft.slug)}
        language={language}
        onChange={(formula) => onChange({ formula })}
      />
    );
  }

  if (draft.type !== "FORMULA") return null;

  return (
    <AggregateEditor
      aggregate={draft.aggregate}
      relations={relations}
      language={language}
      onChange={(aggregate) => onChange({ aggregate })}
    />
  );
}

/**
 * Выражение и слаги полей под ним.
 *
 * Список полей не украшение: в выражении участвуют СЛАГИ, а человек
 * знает поля по подписям — «Цена» и `price_uzs` в голове не совпадают.
 * Клик вставляет слаг туда, где стоит курсор.
 */
function ExpressionEditor({
  formula,
  fields,
  language,
  onChange,
}: {
  formula: string;
  fields: Field[];
  language: string;
  onChange: (formula: string) => void;
}) {
  const { t } = useTranslation();
  const area = useRef<HTMLTextAreaElement>(null);

  const insert = (slug: string) => {
    const element = area.current;
    const from = element?.selectionStart ?? formula.length;
    const to = element?.selectionEnd ?? from;

    onChange(`${formula.slice(0, from)}${slug}${formula.slice(to)}`);

    /*
     * Каретка — за вставленный слаг, и обязательно после перерисовки:
     * значение поля управляемое, и позиция, выставленная до неё,
     * сбрасывается в конец. Иначе вторая вставка уезжает не туда.
     */
    const at = from + slug.length;
    requestAnimationFrame(() => {
      element?.focus();
      element?.setSelectionRange(at, at);
    });
  };

  return (
    <div className="px-2 py-1">
      <span className="mb-0.5 block text-2xs text-fg-muted">{t("formula.expression")}</span>

      <textarea
        ref={area}
        rows={3}
        value={formula}
        spellCheck={false}
        placeholder="(price * count) * 1.12"
        aria-label={t("formula.expression")}
        onChange={(event) => onChange(event.target.value)}
        className="w-full resize-none rounded-md border border-border-strong bg-surface px-2 py-1.5 font-mono text-xs text-fg outline-none focus:border-accent"
      />

      {fields.length > 0 && (
        <>
          <span className="mt-1 mb-1 block text-2xs text-fg-muted">{t("formula.fields")}</span>

          <div className="flex flex-wrap gap-1">
            {fields.map((field) => (
              <button
                key={field.id}
                type="button"
                onClick={() => insert(field.slug)}
                title={localized(field.labels, language, field.label)}
                className="max-w-full rounded-sm transition-opacity hover:opacity-80"
              >
                <Chip>{field.slug}</Chip>
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

/**
 * Агрегат: пять настроек и отбор строк — ровно то же, что спрашивала
 * старая админка, потому что читает их тот же бэкенд.
 *
 * Таблица выбирается не из всех, а из СВЯЗАННЫХ: считать сумму по
 * таблице, с которой эта не связана, бэкенду не по чему — он идёт
 * по связи (`table_from` хранит и слаг, и её id одной строкой).
 */
function AggregateEditor({
  aggregate,
  relations,
  language,
  onChange,
}: {
  aggregate: Aggregate;
  relations: Relation[];
  language: string;
  onChange: (aggregate: Aggregate) => void;
}) {
  const { t } = useTranslation();
  const patch = (next: Partial<Aggregate>) => onChange({ ...aggregate, ...next });

  const slug = tableFromSlug(aggregate.tableFrom);
  /*
   * Поля чужой таблицы: и для «по какому полю считать», и для отбора.
   * Запрос ровно один и только когда таблицу выбрали — хук сам сидит
   * выключенным на пустом слаге.
   */
  // Пустой список колонок — чтобы хук не пошёл ещё и за настройками
  // КАЖДОЙ связи чужой таблицы: здесь нужны только её поля, а связей
  // у неё бывает полтора десятка.
  const { schema } = useTableSchema(slug || undefined, []);

  const linked = relations.filter((relation) => relation.toSlug);
  const summable = schema.fields.filter((field) => field.type !== "LOOKUP");

  return (
    <div className="flex flex-col gap-1.5 px-2 py-1">
      <Labeled label={t("formula.aggregate")}>
        <Select
          value={aggregate.type}
          onChange={(event) => patch({ type: event.target.value as Aggregate["type"] })}
          className="h-7 px-1.5 text-xs"
        >
          <option value="">—</option>
          {AGGREGATES.map((type) => (
            <option key={type} value={type}>
              {t(`formula.${type}` as TranslationKey)}
            </option>
          ))}
        </Select>
      </Labeled>

      <Labeled label={t("formula.table")}>
        <Select
          value={aggregate.tableFrom}
          // Поле и отбор считались по прежней таблице: в новой таких
          // слагов нет, и оставить их значит отправить отбор по полям,
          // которых там не существует.
          onChange={(event) => patch({ tableFrom: event.target.value, field: "", filters: [] })}
          className="h-7 px-1.5 text-xs"
        >
          <option value="">—</option>
          {linked.map((relation) => (
            <option key={relation.id} value={`${relation.toSlug}#${relation.id}`}>
              {relation.toSlug}
            </option>
          ))}
        </Select>
      </Labeled>

      {slug && (
        <Labeled label={t("formula.field")}>
          <Select
            value={aggregate.field}
            onChange={(event) => patch({ field: event.target.value })}
            className="h-7 px-1.5 text-xs"
          >
            <option value="">—</option>
            {summable.map((field) => (
              <option key={field.id} value={field.slug}>
                {localized(field.labels, language, field.label)}
              </option>
            ))}
          </Select>
        </Labeled>
      )}

      <Labeled label={t("formula.rounds")}>
        <Input
          value={aggregate.rounds}
          inputMode="numeric"
          placeholder="0"
          onChange={(event) => patch({ rounds: event.target.value.replace(/\D/g, "") })}
          className="h-7 px-1.5 text-xs tabular-nums"
        />
      </Labeled>

      {slug && (
        <FilterList
          filters={aggregate.filters}
          fields={schema.fields}
          language={language}
          onChange={(filters) => patch({ filters })}
        />
      )}
    </div>
  );
}

/** Подпись над полем ввода в панели. Общая с настройками автозаполнения. */
export function Labeled({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="flex flex-col gap-0.5">
      <span className="text-2xs text-fg-muted">{label}</span>
      {children}
    </label>
  );
}

/**
 * По каким строкам считать.
 *
 * Отбирать можно не по всякому полю: бэкенд сравнивает значение
 * на равенство, и осмысленно это ровно у пяти типов — связь, выбор
 * и два булевых. Тот же список был и в старой админке.
 */
const FILTERABLE = new Set(["LOOKUP", "LOOKUPS", "MULTISELECT", "SWITCH", "CHECKBOX"]);

function FilterList({
  filters,
  fields,
  language,
  onChange,
}: {
  filters: AggregateFilter[];
  /** Поля ЧУЖОЙ таблицы — той, по которой считаем. */
  fields: Field[];
  language: string;
  onChange: (filters: AggregateFilter[]) => void;
}) {
  const { t } = useTranslation();
  const usable = fields.filter((field) => FILTERABLE.has(field.type));

  const patch = (index: number, next: Partial<AggregateFilter>) =>
    onChange(filters.map((filter, i) => (i === index ? { ...filter, ...next } : filter)));

  return (
    <div className="flex flex-col gap-1">
      <div className="flex h-5 items-center gap-1">
        <span className="flex-1 text-2xs text-fg-muted">{t("formula.filters")}</span>

        <button
          type="button"
          onClick={() => onChange([...filters, { key: "", value: null }])}
          disabled={!usable.length}
          aria-label={t("formula.addFilter")}
          title={t("formula.addFilter")}
          className="grid size-5 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-hover hover:text-fg disabled:opacity-40"
        >
          <Icon as={IconPlus} size={14} />
        </button>
      </div>

      {filters.map((filter, index) => {
        const { slug } = filterParts(filter.key);
        const field = usable.find((item) => item.slug === slug);

        return (
          <div key={index} className="flex flex-col gap-1 rounded-md border border-border p-1">
            <div className="flex items-center gap-1">
              <Select
                value={filter.key}
                // Ключ составной — «слаг#ТИП#таблица»: в нём и лежит всё,
                // что нужно и нам, и бэкенду, чтобы понять условие.
                onChange={(event) => patch(index, { key: event.target.value, value: null })}
                className="h-7 min-w-0 flex-1 px-1.5 text-xs"
              >
                <option value="">—</option>
                {usable.map((item) => (
                  <option key={item.id} value={filterKey(item)}>
                    {localized(item.labels, language, item.label)}
                  </option>
                ))}
              </Select>

              <button
                type="button"
                onClick={() => onChange(filters.filter((_, i) => i !== index))}
                aria-label={t("action.delete")}
                className="grid size-7 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-danger-subtle hover:text-danger"
              >
                <Icon as={IconTrash} size={14} />
              </button>
            </div>

            {field && (
              <FilterValue
                field={field}
                value={filter.value}
                language={language}
                onChange={(value) => patch(index, { value })}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

/** Ключ условия ровно в том виде, в каком его читает бэкенд. */
function filterKey(field: Field): string {
  const table = field.raw["table_slug"];
  return `${field.slug}#${field.type}#${typeof table === "string" ? table : ""}`;
}

/**
 * Значение условия. Форма зависит от типа поля, и это не украшение:
 * у выбора это список вариантов, у переключателя — да/нет, у связи —
 * guid'ы строк.
 *
 * ponytail: у связи здесь поле ввода для guid'ов, а не поиск по строкам
 * чужой таблицы, — список строк живёт в features/item, и тянуть его
 * сюда значит замкнуть кольцо между фичами. Уже настроенное условие
 * при этом сохраняется как есть; поиск завести, когда понадобится
 * настраивать такие агрегаты часто.
 */
function FilterValue({
  field,
  value,
  language,
  onChange,
}: {
  field: Field;
  value: unknown;
  language: string;
  onChange: (value: unknown) => void;
}) {
  const { t } = useTranslation();

  if (field.type === "SWITCH" || field.type === "CHECKBOX") {
    return (
      <label className="flex h-7 cursor-pointer items-center gap-2 px-1">
        <Checkbox checked={value === true} onChange={(event) => onChange(event.target.checked)} />
        <span className="truncate text-xs text-fg">{t("action.yes")}</span>
      </label>
    );
  }

  if (field.type === "MULTISELECT") {
    const picked = asList(value);

    return (
      <div className="flex flex-wrap gap-1 p-0.5">
        {[...field.options.values()].map((option) => {
          const on = picked.includes(option.value);

          return (
            <button
              key={option.value}
              type="button"
              onClick={() =>
                onChange(
                  on ? picked.filter((item) => item !== option.value) : [...picked, option.value],
                )
              }
              className={`rounded-sm transition-opacity ${on ? "" : "opacity-40"}`}
            >
              <Chip>{localized(option.labels, language, option.label || option.value)}</Chip>
            </button>
          );
        })}

        {field.options.size === 0 && (
          <span className="px-1 text-2xs text-fg-subtle">{t("table.noOptions")}</span>
        )}
      </div>
    );
  }

  return (
    <Input
      value={asList(value).join(", ")}
      placeholder={t("formula.guids")}
      aria-label={t("formula.guids")}
      onChange={(event) =>
        onChange(
          event.target.value
            .split(",")
            .map((item) => item.trim())
            .filter(Boolean),
        )
      }
      className="h-7 px-1.5 font-mono text-2xs"
    />
  );
}

function asList(value: unknown): string[] {
  if (Array.isArray(value)) return value.map(String);
  return value === null || value === undefined || value === "" ? [] : [String(value)];
}
