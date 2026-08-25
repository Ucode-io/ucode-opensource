import { IconTrash } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Icon } from "@/shared/ui/icon";
import { Select } from "@/shared/ui/input";
import { useTableSchema } from "../api/schema";
import type { CascadeStep } from "../model/cascade";
import { localized, type Relation } from "../model/types";

/**
 * Каскадные списки: цепочка сужения перед выбором.
 *
 * Связь ведёт на «районы», но искать район среди всех районов страны
 * бессмысленно — сперва область, в ней город, в нём район. Здесь эта
 * цепочка и собирается.
 *
 * Собирается она СНИЗУ ВВЕРХ, от цели связи к предку: у таблицы
 * «районы» спрашиваем, куда она сама ссылается, и предлагаем её связи;
 * у выбранной — её связи, и так далее. Показывается потом в обратном
 * порядке — так же, как хранится (см. model/cascade).
 *
 * Список кандидатов собирается из НАШИХ запросов схемы, а не ручкой
 * `/v2/relations/{slug}/cascading`: у неё ветка `case
 * ResourceType_POSTGRESQL` пуста и помечена `// Does Not Implemented`
 * (`gateway/api/handlers/v2/relation.go:79`), то есть на postgres-проекте
 * она отвечает пустотой. Схема таблицы даёт ровно то же — исходящие
 * связи Many2One, — и уже загружена для соседних настроек.
 */
export function CascadeSettings({
  toSlug,
  fieldFrom,
  cascade,
  language,
  onChange,
}: {
  /** Куда ведёт связь. С неё цепочка и начинается. */
  toSlug: string;
  /**
   * Колонка-ссылка В НАШЕЙ таблице. Первое звено пары — это она:
   * у звена `{table_slug: T, field_slug: F}` поле лежит уровнем ниже
   * таблицы.
   */
  fieldFrom: string;
  cascade: CascadeStep[];
  language: string;
  onChange: (cascade: CascadeStep[]) => void;
}) {
  const { t } = useTranslation();

  if (!toSlug) return null;

  /*
   * Цепочка в порядке хранения — от цели вверх. В `cascade` она лежит
   * наоборот (готовой к показу), поэтому здесь разворачивается обратно.
   */
  const chain = [...cascade].reverse();
  const base: CascadeStep = { tableSlug: toSlug, fieldSlug: fieldFrom };
  // Первое звено — сама цель связи; правится оно не здесь, а выбором
  // целевой таблицы, поэтому в списке стоит подписью.
  const levels = chain.length ? chain.slice(1) : [];

  const set = (next: CascadeStep[]) =>
    // Меньше двух звеньев — это не каскад, а просто «куда ведёт связь».
    onChange(next.length ? [base, ...next].reverse() : []);

  return (
    <div className="mt-3 border-t border-border pt-2">
      <p className="px-1 text-2xs text-fg-muted">{t("cascade.title")}</p>
      <p className="px-1 pb-1 text-2xs text-fg-subtle">{t("cascade.hint")}</p>

      <div className="flex items-center gap-1 px-1 pb-1 text-2xs text-fg-subtle">
        <span className="truncate">{toSlug}</span>
      </div>

      {levels.map((step, index) => (
        <CascadeLevel
          key={index}
          /* Кандидаты берутся у таблицы ПРЕДЫДУЩЕГО звена: это она
             ссылается вверх. */
          fromSlug={(levels[index - 1] ?? base).tableSlug}
          step={step}
          language={language}
          onChange={(next) => set([...levels.slice(0, index), next])}
          onRemove={() => set(levels.slice(0, index))}
        />
      ))}

      <CascadeLevel
        fromSlug={(levels[levels.length - 1] ?? base).tableSlug}
        language={language}
        onChange={(next) => set([...levels, next])}
      />
    </div>
  );
}

/**
 * Один уровень: куда ссылается таблица предыдущего звена.
 *
 * Схема запрашивается своим хуком на каждый уровень — их единицы,
 * а собрать все разом нечем: следующая таблица известна только после
 * выбора в предыдущей.
 */
function CascadeLevel({
  fromSlug,
  step,
  language,
  onChange,
  onRemove,
}: {
  fromSlug: string;
  /** Уже выбранное звено. Нет — это строка «добавить уровень». */
  step?: CascadeStep;
  language: string;
  onChange: (step: CascadeStep) => void;
  onRemove?: () => void;
}) {
  const { t } = useTranslation();
  const { schema } = useTableSchema(fromSlug || undefined, []);

  /*
   * Годятся только исходящие Many2One: у них колонка-ссылка лежит
   * в САМОЙ таблице, и ею отбирается следующий список. У входящей
   * связи ссылка в чужой строке — отбирать по ней нечем.
   */
  const options = schema.relations.filter(
    (item) => item.direction === "outgoing" && item.toSlug && item.fieldFrom,
  );

  if (!options.length && !step) return null;

  const value = step ? `${step.tableSlug}#${step.fieldSlug}` : "";

  return (
    <div className="flex items-center gap-1 px-1 pb-1">
      <span className="text-2xs text-fg-subtle">↑</span>

      <Select
        value={value}
        onChange={(event) => {
          const [tableSlug = "", fieldSlug = ""] = event.target.value.split("#");
          if (tableSlug && fieldSlug) onChange({ tableSlug, fieldSlug });
        }}
        className="h-7 min-w-0 flex-1 px-1.5 text-xs"
      >
        <option value="">{t("cascade.addLevel")}</option>
        {options.map((item) => (
          <option key={item.id} value={`${item.toSlug}#${item.fieldFrom}`}>
            {label(item, language)}
          </option>
        ))}
      </Select>

      {onRemove && (
        <button
          type="button"
          onClick={onRemove}
          aria-label={t("action.delete")}
          className="grid size-6 shrink-0 place-items-center rounded text-fg-subtle transition-colors hover:bg-danger-subtle hover:text-danger"
        >
          <Icon as={IconTrash} size={12} />
        </button>
      )}
    </div>
  );
}

/** Имя связи, а не слаг: у двух связей на одну таблицу слаг общий. */
function label(relation: Relation, language: string): string {
  const name = relation.title || localized(relation.toLabels, language, relation.toLabel);
  return name && name !== relation.toSlug ? `${relation.toSlug} · ${name}` : relation.toSlug;
}
