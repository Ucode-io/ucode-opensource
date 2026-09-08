#!/usr/bin/env node
/**
 * routeTree.gen.ts генерируется плагином при dev/build, но tsc в npm run build
 * стартует раньше vite. Поэтому генерим отдельным шагом.
 */
import { Generator, getConfig } from "@tanstack/router-generator";

const config = getConfig({ target: "react", autoCodeSplitting: true }, process.cwd());
await new Generator({ config, root: process.cwd() }).run();
console.log("routeTree.gen.ts готов");
