import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useDataLanguages } from "@/features/workspace";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { Dropdown } from "@/shared/ui/dropdown";
import { Field, Input } from "@/shared/ui/input";
import { LanguageTabs } from "@/shared/ui/language-tabs";
import { ImagePicker } from "./ImagePicker";
import {
  useIconCollections,
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
  const { options: currencies } = useProjectOptions("CURRENCY");
  const update = useUpdateProject(languages);
  const { languages: dataLanguages, current, setCurrent } = useDataLanguages();

  const [title, setTitle] = useState("");
  const [languageIds, setLanguageIds] = useState<string[]>([]);
  const [timezoneId, setTimezoneId] = useState("");
  const [currencyId, setCurrencyId] = useState("");
  const [logo, setLogo] = useState("");
  const [iconCategories, setIconCategories] = useState<string[]>([]);
  /* Языков в справочнике под две сотни — без поиска это стена флажков. */
  const [query, setQuery] = useState("");

  useEffect(() => {
    if (!project) return;
    setTitle(project.title);
    setLanguageIds(project.languageIds);
    setTimezoneId(project.timezoneId);
    setCurrencyId(project.currencyId);
    setLogo(project.logo);
    setIconCategories(project.iconCategories);
  }, [project]);

  if (isLoading || !project) {
    return <p className="text-sm text-fg-subtle">{t("common.loading")}</p>;
  }

  const changed =
    title !== project.title ||
    timezoneId !== project.timezoneId ||
    currencyId !== project.currencyId ||
    logo !== project.logo ||
    languageIds.length !== project.languageIds.length ||
    languageIds.some((id) => !project.languageIds.includes(id)) ||
    iconCategories.length !== project.iconCategories.length ||
    iconCategories.some((value) => !project.iconCategories.includes(value));

  const toggle = (id: string) =>
    setLanguageIds((current) =>
      current.includes(id) ? current.filter((item) => item !== id) : [...current, id],
    );

  return (
    /* Ширину берём от окна — см. ProfileSettings: узкая колонка в широком
       окне настроек оставляла пустой правый край. */
    <div className="flex w-full max-w-5xl flex-col gap-6">
      <ImagePicker
        value={logo}
        letter={(title || project.id).slice(0, 1).toUpperCase()}
        label={t("settings.logo")}
        hint={t("settings.logoHint")}
        onChange={setLogo}
      />

      <div className="grid grid-cols-2 gap-4 lg:grid-cols-3">
        <Field label={t("settings.projectName")}>
          <Input value={title} onChange={(event) => setTitle(event.target.value)} />
        </Field>

        <Field label={t("settings.timezone")}>
          <Dropdown
            value={timezoneId}
            placeholder={t("settings.noTimezone")}
            items={timezones.map((zone) => ({ value: zone.id, label: zone.name }))}
            onChange={setTimezoneId}
          />
        </Field>

        {/* Валюта проекта: ею подписаны денежные поля. Тот же справочник,
            что у языков и поясов, — только с другим типом. */}
        <Field label={t("settings.currency")}>
          <Dropdown
            value={currencyId}
            placeholder={t("settings.noCurrency")}
            items={currencies.map((item) => ({ value: item.id, label: item.name }))}
            onChange={setCurrencyId}
          />
        </Field>
      </div>

      {/*
        Языки и наборы значков — рядом: оба со своей прокруткой,
        и друг под другом они гнали страницу вниз на два экрана.

        Строки общие для обеих секций (subgrid): подсказка слева
        занимает две строки, справа одну, и без общей сетки поля поиска
        и рамки списков стоят на разной высоте — колонки выглядят
        косыми.
      */}
      <div className="grid gap-x-6 gap-y-2 lg:grid-cols-2 lg:grid-rows-[auto_auto_auto_1fr]">
        <section className="grid content-start gap-2 lg:row-span-4 lg:grid-rows-subgrid">
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
        <div className="grid h-56 auto-rows-min gap-1 overflow-y-auto rounded-md border border-border p-2">
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

        <IconCategories value={iconCategories} onChange={setIconCategories} />
      </div>

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
          onClick={() =>
            update.mutate({
              project,
              draft: { title, languageIds, timezoneId, currencyId, iconCategories, logo },
            })
          }
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

/**
 * Наборы значков, из которых выбирают иконку пункта меню.
 *
 * Список приходит не от бэкенда, а прямо из iconify
 * (api.iconify.design/collections) — оттуда же берутся и сами значки.
 * Так это устроено и в старой админке: свой справочник наборов ucode
 * не держит.
 *
 * Значение — `<префикс>#<имя набора>`: префикс нужен запросу значков,
 * имя — человеку в списке. Формат не наш, его читает старая админка,
 * и менять его значит разойтись с ней на одних и тех же данных.
 *
 * Запрос уходит один раз на открытие настроек и живёт в кэше сутки:
 * набор коллекций iconify меняется несколько раз в год.
 */
function IconCategories({
  value,
  onChange,
}: {
  value: string[];
  onChange: (next: string[]) => void;
}) {
  const { t } = useTranslation();
  const { collections, isLoading } = useIconCollections();
  const [query, setQuery] = useState("");

  const chosen = new Set(value);
  const shown = collections
    .filter((item) => chosen.has(item.value) || matches(item.label, query))
    .slice(0, 200);

  return (
    <section className="grid content-start gap-2 lg:row-span-4 lg:grid-rows-subgrid">
      <h3 className="text-sm font-medium">{t("settings.iconCategories")}</h3>
      <p className="text-xs text-fg-subtle">{t("settings.iconCategoriesHint")}</p>

      <Input
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        placeholder={t("settings.searchIconCategory")}
        aria-label={t("settings.searchIconCategory")}
      />

      <div className="grid h-56 auto-rows-min gap-1 overflow-y-auto rounded-md border border-border p-2">
        {isLoading && <p className="p-1 text-xs text-fg-subtle">{t("common.loading")}</p>}

        {shown.map((item) => (
          <label
            key={item.value}
            className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1 transition-colors hover:bg-surface-hover"
          >
            <Checkbox
              checked={chosen.has(item.value)}
              onChange={() =>
                onChange(
                  chosen.has(item.value)
                    ? value.filter((current) => current !== item.value)
                    : [...value, item.value],
                )
              }
            />
            <span className="min-w-0 truncate text-sm text-fg">{item.label}</span>
          </label>
        ))}
      </div>
    </section>
  );
}

function matches(label: string, query: string): boolean {
  return !query.trim() || label.toLowerCase().includes(query.trim().toLowerCase());
}
