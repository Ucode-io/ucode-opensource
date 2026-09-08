import {
  IconApi,
  IconBrandGithub,
  IconBrandGitlab,
  IconChartBar,
  IconChartHistogram,
  IconDatabase,
  IconMail,
  IconMailFast,
  IconMessage2,
  IconPlug,
  IconVideo,
} from "@tabler/icons-react";
import { Icon } from "@/shared/ui/icon";

/**
 * Значок типа ресурса.
 *
 * Отдельным файлом, потому что нужен в двух местах — в строке списка
 * и в плитке выбора типа, — и потому что это единственное место, где
 * имя типа превращается в картинку.
 *
 * Логотипов вендоров тут нет намеренно: своя картинка на каждый тип
 * означала бы двадцать файлов в public и своё поведение в тёмной теме.
 * Значок говорит О ПОВОДЕ (письмо, репозиторий, график), а имя службы
 * написано рядом словами.
 */
const ICONS: Record<string, typeof IconApi> = {
  SMS: IconMessage2,
  SMTP: IconMail,
  MAILCHIMP: IconMailFast,
  GITHUB: IconBrandGithub,
  GITLAB: IconBrandGitlab,
  SUPERSET: IconChartBar,
  METABASE: IconChartHistogram,
  TRANSCODER: IconVideo,
  REST: IconApi,
  MONGODB: IconDatabase,
  CLICKHOUSE: IconDatabase,
  POSTGRESQL: IconDatabase,
};

export function ResourceIcon({ kind, size = 16 }: { kind: string; size?: number }) {
  return <Icon as={ICONS[kind] ?? IconPlug} size={size} />;
}
