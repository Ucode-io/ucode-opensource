#!/usr/bin/env node
/**
 * Swagger 2.0 бэкенда → TypeScript.
 *
 * openapi-typescript понимает только OpenAPI 3, а gateway отдаёт Swagger 2.0
 * (181 path, 222 definition), поэтому сначала конвертируем.
 *
 * Сгенерированный файл в git не коммитится и руками не правится. Если типы
 * врут — правится swagger на бэкенде либо доменный тип в features/<имя>/model/.
 */
import { writeFile, readFile, mkdir } from "node:fs/promises";
import { resolve, dirname } from "node:path";
import openapiTS, { astToString } from "openapi-typescript";
import { convertObj } from "swagger2openapi";

const SWAGGER = process.env.UCODE_SWAGGER ?? resolve(
  import.meta.dirname,
  "../../ucode_backend/ucode_go_admin_api_gateway/api/docs/swagger.json",
);
const OUT = resolve(import.meta.dirname, "../src/shared/api/schema.d.ts");

const swagger2 = JSON.parse(await readFile(SWAGGER, "utf8"));
const { openapi } = await convertObj(swagger2, { patch: true, warnOnly: true });

const ast = await openapiTS(openapi);
await mkdir(dirname(OUT), { recursive: true });
await writeFile(OUT, `// Сгенерировано npm run gen:api. Не редактировать.\n${astToString(ast)}`);

console.log(`${OUT}: ${Object.keys(openapi.paths ?? {}).length} путей`);
