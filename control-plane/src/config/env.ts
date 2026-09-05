import { z } from 'zod';

const environmentSchema = z.object({
  PORT: z.coerce.number().int().min(1).max(65_535).default(3001),
  DATABASE_URL: z.string().url().optional(),
  AWARENOW_CONTROL_TOKEN: z.string().trim().min(1).optional(),
  CONTROL_PLANE_BASE_URL: z.string().url().optional(),
});

export type ControlPlaneEnvironment = {
  port: number;
  databaseUrl?: string;
  controlToken?: string;
  controlPlaneBaseUrl?: string;
};

/**
 * Validates runtime settings without logging credential material.
 */
export function loadEnvironment(environment: NodeJS.ProcessEnv = process.env): ControlPlaneEnvironment {
  const parsed = environmentSchema.parse(environment);

  return {
    port: parsed.PORT,
    databaseUrl: parsed.DATABASE_URL,
    controlToken: parsed.AWARENOW_CONTROL_TOKEN,
    controlPlaneBaseUrl: parsed.CONTROL_PLANE_BASE_URL,
  };
}
