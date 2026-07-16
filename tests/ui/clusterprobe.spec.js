const { expect, request, test } = require('@playwright/test');

const apiURL = process.env.API_URL || 'http://127.0.0.1:8080';

async function pollJSON(getter, accept, timeoutMs = 45_000) {
  const deadline = Date.now() + timeoutMs;
  let last;
  while (Date.now() < deadline) {
    last = await getter();
    if (accept(last)) {
      return last;
    }
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
  throw new Error(`timed out waiting for expected response; last=${JSON.stringify(last)}`);
}

test.describe.serial('ClusterProbe browser smoke', () => {
  let api;

  test.beforeAll(async () => {
    api = await request.newContext({ baseURL: apiURL });
  });

  test.afterAll(async () => {
    await api.dispose();
  });

  test('dashboard renders live status and metrics cards', async ({ page }) => {
    const status = await api.get('/api/v1/status');
    expect(status.ok()).toBeTruthy();
    await expect(status.json()).resolves.toEqual(expect.objectContaining({ status: 'ok' }));

    await page.goto('/dashboard');
    await expect(page).toHaveTitle(/Dashboard \| ClusterProbe/);
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible();
    await expect(page.getByText('Live', { exact: true })).toBeVisible();
    await expect(page.getByText('Ops/sec')).toBeVisible();
    await expect(page.getByText('Active Scenarios')).toBeVisible();
    await expect(page.getByText('Queue Depth')).toBeVisible();
  });

  test('scenario created through API completes and appears in the UI', async ({ page }) => {
    const name = `ui-smoke-${Date.now()}`;
    const create = await api.post('/api/v1/scenarios', {
      data: {
        name,
        profile: {
          rps: 1,
          duration: 3_000_000_000,
          payload_size_bytes: 0,
          concurrency: 1,
          target_queue: 'workload.high',
          workload_type: 'db_write',
        },
      },
    });
    expect(create.status()).toBe(202);
    const scenario = await create.json();
    expect(scenario.id).toBeTruthy();

    await pollJSON(
      async () => {
        const response = await api.get(`/api/v1/scenarios/${scenario.id}`);
        expect(response.ok()).toBeTruthy();
        return response.json();
      },
      (body) => body.status === 'completed',
    );

    await page.goto('/scenarios');
    await expect(page).toHaveTitle(/Scenarios \| ClusterProbe/);
    await expect(page.locator(`#scenario-${scenario.id}`)).toContainText(name);
    await expect(page.locator(`#scenario-${scenario.id}`)).toContainText('completed');

    await page.locator(`#scenario-${scenario.id} a`, { hasText: name }).click();
    await expect(page).toHaveTitle(/Scenario \| ClusterProbe/);
    await expect(page.getByRole('heading', { name: `Scenario ${name}` })).toBeVisible();
    const lifecycle = page.locator('section.card').filter({ hasText: 'Lifecycle Events' });
    await expect(lifecycle).toBeVisible();
    await expect(lifecycle.getByText('queued')).toBeVisible();
    await expect(lifecycle.getByText('completed')).toBeVisible();
  });

  test('chaos experiment reaches completed status and appears in the UI', async ({ page }) => {
    const name = `ui-stress-${Date.now()}`;
    const create = await api.post('/api/v1/chaos/experiments', {
      data: {
        name,
        scenario: 'ui-chaos-smoke',
        config: {
          type: 'stress',
          target: 'app.kubernetes.io/component=worker',
          duration: '10s',
          workers: '1',
          load: '5',
        },
      },
    });
    expect(create.status()).toBe(202);
    const experiment = await create.json();
    expect(experiment.id).toBe(name);

    await pollJSON(
      async () => {
        const response = await api.get(`/api/v1/chaos/experiments/${experiment.id}`);
        expect(response.ok()).toBeTruthy();
        return response.json();
      },
      (body) => body.status === 'completed',
      30_000,
    );

    await page.goto('/chaos');
    await expect(page).toHaveTitle(/Chaos Experiments \| ClusterProbe/);
    await expect(page.locator(`#experiment-${experiment.id}`)).toContainText(name);
    await expect(page.locator(`#experiment-${experiment.id}`)).toContainText('completed');

    await page.locator(`#experiment-${experiment.id} a`, { hasText: name }).first().click();
    await expect(page).toHaveTitle(/Chaos Experiment \| ClusterProbe/);
    await expect(page.getByRole('heading', { name: `Chaos Experiment ${name}` })).toBeVisible();
    const config = page.locator('section.card').filter({ hasText: 'Experiment Config' });
    await expect(config).toBeVisible();
    await expect(config.getByRole('cell', { name: 'stress' })).toBeVisible();
    await expect(config.getByRole('cell', { name: 'app.kubernetes.io/component=worker' })).toBeVisible();

    const deleted = await api.delete(`/api/v1/chaos/experiments/${experiment.id}`);
    expect([204, 404]).toContain(deleted.status());
  });

  test('logs page renders stream container', async ({ page }) => {
    await page.goto('/logs');
    await expect(page).toHaveTitle(/Logs \| ClusterProbe/);
    await expect(page.getByRole('heading', { name: 'Logs' })).toBeVisible();
    await expect(page.locator('#log-stream')).toBeVisible();
  });
});
