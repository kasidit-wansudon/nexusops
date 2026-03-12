const request = require("supertest");
const app = require("./index");

describe("Health endpoint", () => {
  it("GET /health returns 200 with status healthy", async () => {
    const res = await request(app).get("/health");

    expect(res.status).toBe(200);
    expect(res.body.status).toBe("healthy");
    expect(res.body.service).toBe("node-express-api");
    expect(res.body).toHaveProperty("uptime");
  });
});

describe("Users API", () => {
  let createdUserId;

  it("GET /api/users returns an empty list initially", async () => {
    const res = await request(app).get("/api/users");

    expect(res.status).toBe(200);
    expect(Array.isArray(res.body)).toBe(true);
  });

  it("POST /api/users creates a new user", async () => {
    const res = await request(app)
      .post("/api/users")
      .send({ name: "Alice", email: "alice@example.com" });

    expect(res.status).toBe(201);
    expect(res.body).toHaveProperty("id");
    expect(res.body.name).toBe("Alice");
    expect(res.body.email).toBe("alice@example.com");
    expect(res.body).toHaveProperty("created_at");

    createdUserId = res.body.id;
  });

  it("POST /api/users returns 400 when name is missing", async () => {
    const res = await request(app)
      .post("/api/users")
      .send({ email: "bob@example.com" });

    expect(res.status).toBe(400);
    expect(res.body.error).toMatch(/required/);
  });

  it("POST /api/users returns 400 when email is missing", async () => {
    const res = await request(app)
      .post("/api/users")
      .send({ name: "Bob" });

    expect(res.status).toBe(400);
    expect(res.body.error).toMatch(/required/);
  });

  it("GET /api/users/:id returns the created user", async () => {
    const res = await request(app).get(`/api/users/${createdUserId}`);

    expect(res.status).toBe(200);
    expect(res.body.id).toBe(createdUserId);
    expect(res.body.name).toBe("Alice");
  });

  it("GET /api/users/:id returns 404 for unknown id", async () => {
    const res = await request(app).get("/api/users/nonexistent");

    expect(res.status).toBe(404);
    expect(res.body.error).toMatch(/not found/i);
  });

  it("PUT /api/users/:id updates user fields", async () => {
    const res = await request(app)
      .put(`/api/users/${createdUserId}`)
      .send({ name: "Alice Updated" });

    expect(res.status).toBe(200);
    expect(res.body.name).toBe("Alice Updated");
    expect(res.body.email).toBe("alice@example.com");
  });

  it("PUT /api/users/:id returns 404 for unknown id", async () => {
    const res = await request(app)
      .put("/api/users/nonexistent")
      .send({ name: "Ghost" });

    expect(res.status).toBe(404);
  });

  it("DELETE /api/users/:id removes the user", async () => {
    const res = await request(app).delete(`/api/users/${createdUserId}`);

    expect(res.status).toBe(204);
  });

  it("DELETE /api/users/:id returns 404 after deletion", async () => {
    const res = await request(app).delete(`/api/users/${createdUserId}`);

    expect(res.status).toBe(404);
  });

  it("GET /api/users lists remaining users after deletion", async () => {
    const res = await request(app).get("/api/users");

    expect(res.status).toBe(200);
    const ids = res.body.map((u) => u.id);
    expect(ids).not.toContain(createdUserId);
  });
});
