import { resolve } from "node:path";
import document from "../../infrastructure/assets/service-sdk/v2/http-contract.json";

export const projectRoot = resolve(import.meta.dirname, "../..");

export const contractFile = resolve(
  projectRoot,
  "infrastructure/assets/service-sdk/v2/http-contract.json",
);

export type Endpoint = { method: string; path: string };
export type Problem = { status: number; code: string };

// JSON imports infer the published artifact's complete shape. Widen only the
// dictionaries whose keys are intentionally looked up at runtime.
export const contract = {
  ...document,
  service: { ...document.service, endpoints: document.service.endpoints as typeof document.service.endpoints & Record<string, Endpoint> },
  problems: document.problems as typeof document.problems & Record<string, Problem>,
  errors: document.errors as typeof document.errors & Record<string, Problem>,
  outcomes: document.outcomes as typeof document.outcomes & Record<string, string[]>,
  outcomeProblems: document.outcomeProblems as typeof document.outcomeProblems & Record<string, string>,
};

export const endpointNames: string[] = Object.keys(contract.service.endpoints);
export const problemKeys: string[] = Object.keys(contract.problems);
export const errorKeys: string[] = Object.keys(contract.errors);
export const outcomeFamilies: string[] = Object.keys(contract.outcomes);
export const outcomeNames: string[] = Object.values(contract.outcomes).flat();
export const successOutcomes: string[] = contract.successOutcomes;

export function endpoint(name: string): Endpoint {
  const found = contract.service.endpoints[name];
  if (!found) {
    throw new Error(`the contract registers no endpoint named ${name}`);
  }
  return found;
}

export function path(name: string): string {
  return endpoint(name).path;
}

export function method(name: string): string {
  return endpoint(name).method;
}

export function transportProblem(key: string): Problem {
  const found = contract.problems[key];
  if (!found) {
    throw new Error(`the contract registers no transport problem named ${key}`);
  }
  return found;
}

export function error(key: string): Problem {
  const found = contract.errors[key];
  if (!found) {
    throw new Error(`the contract registers no error named ${key}`);
  }
  return found;
}

export function problemOfOutcome(outcome: string): Problem {
  const key = contract.outcomeProblems[outcome];
  if (typeof key !== "string") {
    throw new Error(`the contract registers no problem for outcome ${outcome}`);
  }
  return error(key);
}

export function statusOfOutcome(outcome: string): number {
  return problemOfOutcome(outcome).status;
}

export function codeOfOutcome(outcome: string): string {
  return problemOfOutcome(outcome).code;
}

export function isSuccessOutcome(outcome: string): boolean {
  return successOutcomes.includes(outcome);
}

export function outcomesOf(family: string): string[] {
  const found = contract.outcomes[family];
  if (!found) {
    throw new Error(`the contract registers no outcome family named ${family}`);
  }
  return found;
}

export const mediaType = {
  json: contract.service.responses.contentTypes.json,
  metrics: contract.service.responses.contentTypes.metrics,
  registration: contract.identity.registration.mediaType,
  reloadRequest: contract.service.reloadRequest.mediaType,
  reloadAcknowledgement: contract.service.reloadAcknowledgement.mediaType,
  readiness: contract.service.readiness.mediaType,
  manifest: contract.service.manifest.mediaType,
  configurationSchema: contract.service.configurationSchema.mediaType,
  configPull: contract.core.configPull.responseMediaType,
};

export const maximumBytes = {
  registration: contract.identity.registration.maximumBytes,
  reloadRequest: contract.service.reloadRequest.maximumBytes,
  reloadAcknowledgement: contract.service.reloadAcknowledgement.maximumBytes,
  readiness: contract.service.readiness.maximumBytes,
  manifest: contract.service.manifest.maximumBytes,
  configurationSchema: contract.service.configurationSchema.maximumBytes,
  metadata: contract.service.maximumMetadataBytes,
  configPull: contract.core.configPull.maximumBytes,
};

export const responseHeader: Record<string, string> = contract.core.configPull.responseHeaders;

export const serviceResponses = contract.service.responses;

export const metrics = contract.service.responses.metrics;

export const registrationRequired: string[] = contract.identity.registration.required;

export const health = {
  status: contract.service.responses.health.status,
  body: contract.service.responses.health.body as Record<string, unknown>,
};

export function requiredFields(document: string): string[] {
  const documents: Record<string, { required: string[] }> = {
    reloadRequest: contract.service.reloadRequest,
    reloadAcknowledgement: contract.service.reloadAcknowledgement,
    readiness: contract.service.readiness,
  };
  const found = documents[document];
  if (!found) {
    throw new Error(`the contract registers no document named ${document}`);
  }
  return found.required;
}

export function pullPath(generation: string): string {
  return contract.core.configPull.pathTemplate.replace(
    "{generation}",
    encodeURIComponent(generation),
  );
}
