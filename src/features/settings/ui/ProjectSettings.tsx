import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useDataLanguages } from "@/features/workspace";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { Field, Input, Select } from "@/shared/ui/input";
import { LanguageTabs } from "@/shared/ui/language-tabs";
import {
  useLanguageOptions,
  useProject,
  useProjectOptions,
  useUpdateProject,
  type LanguageOption,
} from "../api/project";

/**
 * Настройки проекта: имя, языки данных, часовой пояс.
 *
 * Языки — это [[Data Language]]: на них размечены подписи полей, имена
 * view и мультиязычные колонки. Убрать язык из проекта значит осиротить
 * все подписи с его кодом, поэтому список отмечается флажками, а не
 * правится строкой.
 *
 * Здесь же выбор языка, на котором данные ПОКАЗАНЫ: он личный и в проект
 * не уезжает, но стоит рядом с набором — иначе человек ищет его в двух
 * разных местах.
 */
export function ProjectSettings() {
  const { t } = useTranslation();
  const { project, isLoading } = useProject();
  const { languages } = useLanguageOptions();
  const { options: timezones } = useProjectOptions("TIMEZONE");
  const update = useUpdateProject(languages);
  const { languages: dataLanguages, current, setCurrent } = useDataLanguages();

  const [title, setTitle] = useState("");
  const [languageIds, setLanguageIds] = useState<string[]>([]);
  const [timezoneId, setTimezoneId] = useState("");
  /* Языков в справочнике под две сотни — без поиска это стена флажков. */
  const [query, setQuery] = useState("");

  useEffect(() => {
    if (!project) return;
    setTitle(project.title);
    setLanguageIds(project.languageIds);
    setTimezoneId(project.timezoneId);
  }, [project]);

  if (isLoading || !project) {
    return <p className="text-sm text-fg-subtle">{t("common.loading")}</p>;
  }

  const changed =
    title !== project.title ||
    timezoneId !== project.timezoneId ||
    languageIds.length !== project.languageIds.length ||
    languageIds.some((id) => !project.languageIds.includes(id));

  const toggle = (id: string) =>
    setLanguageIds((current) =>
      current.includes(id) ? current.filter((item) => item !== id) : [...current, id],
    );

  return (
    <div className="flex max-w-2xl flex-col gap-6">
      <div className="grid grid-cols-2 gap-4">
        <Field label={t("settings.projectName")}>
          <Input value={title} onChange={(event) => setTitle(event.target.value)} />
        </Field>

        <Field label={t("settings.timezone")}>
          <Select value={timezoneId} onChange={(event) => setTimezoneId(event.target.value)}>
            <option value="">{t("settings.noTimezone")}</option>
            {timezones.map((zone) => (
              <option key={zone.id} value={zone.id}>
                {zone.name}
              </option>
            ))}
          </Select>
        </Field>
      </div>

      <section className="flex flex-col gap-2">
        <h3 className="text-sm font-medium">{t("settings.dataLanguages")}</h3>
        <p className="text-xs text-fg-subtle">{t("settings.dataLanguagesHint")}</p>

        <Input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={t("settings.searchLanguage")}
          aria-label={t("settings.searchLanguage")}
        />

        {/* Выбранные всегда сверху и всегда видны: иначе поиск прячет
            то, что человек только что отметил. */}
        <div className="grid max-h-56 grid-cols-2 gap-1 overflow-y-auto rounded-md border border-border p-2">
          {matching(languages, languageIds, query).map((language) => (
            <label
              key={language.id}
              className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1 transition-colors hover:bg-surface-hover"
            >
              <Checkbox
                checked={languageIds.includes(language.id)}
                onChange={() => toggle(language.id)}
              />
              <span className="min-w-0 truncate text-sm text-fg">
                {language.native_name || language.name}
              </span>
              <span className="ml-auto shrink-0 text-2xs text-fg-subtle">
                {language.short_name}
              </span>
            </label>
          ))}
        </div>
      </section>

      {/* Личный выбор, а не настройка проекта: он не уезжает на сервер
          и не меняет данные — только то, какой языковой вариант показан. */}
      {dataLanguages.length > 1 && (
        <section className="flex items-center gap-3 rounded-md bg-surface-active/50 px-3 py-2">
          <div className="min-w-0 flex-1">
            <p className="text-sm text-fg">{t("settings.shownLanguage")}</p>
            <p className="text-2xs text-fg-subtle">{t("settings.shownLanguageHint")}</p>
          </div>

          <LanguageTabs languages={dataLanguages} value={current} onChange={setCurrent} />
        </section>
      )}

      <div className="flex justify-end">
        <Button
          disabled={!changed || update.isPending}
          onClick={() => update.mutate({ project, draft: { title, languageIds, timezoneId } })}
        >
          {t("action.save")}
        </Button>
      </div>
    </div>
  );
}

/**
 * Языки, подходящие под поиск. Отмеченные показываются всегда и первыми:
 * список из двух сотен строк иначе прячет собственный выбор.
 */
function matching(
  languages: LanguageOption[],
  selected: string[],
  query: string,
): LanguageOption[] {
  const needle = query.trim().toLowerCase();

  const chosen = languages.filter((language) => selected.includes(language.id));
  const rest = languages.filter(
    (language) =>
      !selected.includes(language.id) &&
      (!needle ||
        language.name.toLowerCase().includes(needle) ||
        language.native_name.toLowerCase().includes(needle) ||
        language.short_name.toLowerCase().includes(needle)),
  );

  return [...chosen, ...(needle ? rest : rest.slice(0, VISIBLE_LANGUAGES))];
}

/**
 * Сколько языков показать без поиска. Полный справочник — 184 строки,
 * и прокручивать их до нужного дольше, чем набрать две буквы.
 */
const VISIBLE_LANGUAGES = 30;
