import { IconPlus, IconTrash } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Icon } from "@/shared/ui/icon";
import { Input } from "@/shared/ui/input";
import type { ResourceVariable } from "../../api/resources";

/**
 * Переменные ресурса — пары «имя-значение».
 *
 * Ими живёт тип REST: своих настроек у него нет вовсе, а функции
 * достают значение по имени. Поэтому REST без переменных бесполезен,
 * и форма заводит первую строку сразу.
 *
 * Строка без имени не сохраняется (см. api/resources): пустое поле —
 * это заготовка, а не переменная.
 */
export function ResourceVariables({
  variables,
  onChange,
}: {
  variables: ResourceVariable[];
  onChange: (variables: ResourceVariable[]) => void;
}) {
  const { t } = useTranslation();

  const patch = (index: number, part: Partial<ResourceVariable>) =>
    onChange(variables.map((variable, i) => (i === index ? { ...variable, ...part } : variable)));

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-baseline justify-between gap-2">
        <span className="text-xs font-medium text-fg-muted">{t("resources.variables")}</span>
        <span className="text-xs text-fg-subtle">{t("resources.variablesHint")}</span>
      </div>

      {variables.map((variable, index) => (
        /* Ключ по месту в списке: строки только добавляют и убирают,
           а значение поля целиком приходит сверху — своего состояния,
           которое могло бы переехать на соседа, у input нет. */
        <div key={index} className="flex items-center gap-2">
          <Input
            value={variable.key}
            placeholder={t("resources.variableKey")}
            onChange={(event) => patch(index, { key: event.target.value })}
            className="font-mono text-xs"
          />

          <Input
            value={variable.value}
            placeholder={t("resources.variableValue")}
            onChange={(event) => patch(index, { value: event.target.value })}
            className="font-mono text-xs"
          />

          <button
            type="button"
            onClick={() => onChange(variables.filter((_, i) => i !== index))}
            aria-label={t("action.delete")}
            title={t("action.delete")}
            className="grid size-8 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-danger-subtle hover:text-danger"
          >
            <Icon as={IconTrash} size={14} />
          </button>
        </div>
      ))}

      <button
        type="button"
        onClick={() => onChange([...variables, { id: "", key: "", value: "" }])}
        className="inline-flex h-8 items-center gap-1.5 self-start rounded-md px-2 text-xs text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
      >
        <Icon as={IconPlus} size={14} />
        {t("resources.addVariable")}
      </button>
    </div>
  );
}
