const express = require("express");
const helmet = require("helmet");
const morgan = require("morgan");
const cors = require("cors");
const { v4: uuidv4 } = require("uuid");

const app = express();
const PORT = process.env.PORT || 3000;

// ---------------------------------------------------------------------------
// Middleware
// ---------------------------------------------------------------------------

app.use(helmet());
app.use(cors());
app.use(express.json());

// Structured JSON request logging.
app.use(
  morgan((tokens, req, res) =>
    JSON.stringify({
      timestamp: new Date().toISOString(),
      method: tokens.method(req, res),
      path: tokens.url(req, res),
      status: Number(tokens.status(req, res)),
      duration_ms: Number(tokens["response-time"](req, res)),
      content_length: tokens.res(req, res, "content-length") || "0",
    })
  )
);

// ---------------------------------------------------------------------------
// In-memory data store
// ---------------------------------------------------------------------------

const users = new Map();

// ---------------------------------------------------------------------------
// Routes
// ---------------------------------------------------------------------------

// Health check endpoint — returns service status for NexusOps deploy probes.
app.get("/health", (_req, res) => {
  res.json({
    status: "healthy",
    service: "node-express-api",
    uptime: process.uptime(),
  });
});

// List all users.
app.get("/api/users", (_req, res) => {
  const list = Array.from(users.values());
  res.json(list);
});

// Get a single user by ID.
app.get("/api/users/:id", (req, res) => {
  const user = users.get(req.params.id);
  if (!user) {
    return res.status(404).json({ error: "User not found" });
  }
  res.json(user);
});

// Create a new user.
app.post("/api/users", (req, res) => {
  const { name, email } = req.body;

  if (!name || !email) {
    return res.status(400).json({ error: "name and email are required" });
  }

  const user = {
    id: uuidv4(),
    name,
    email,
    created_at: new Date().toISOString(),
  };

  users.set(user.id, user);
  res.status(201).json(user);
});

// Update an existing user.
app.put("/api/users/:id", (req, res) => {
  const existing = users.get(req.params.id);
  if (!existing) {
    return res.status(404).json({ error: "User not found" });
  }

  const { name, email } = req.body;
  const updated = {
    ...existing,
    name: name || existing.name,
    email: email || existing.email,
  };

  users.set(updated.id, updated);
  res.json(updated);
});

// Delete a user.
app.delete("/api/users/:id", (req, res) => {
  if (!users.has(req.params.id)) {
    return res.status(404).json({ error: "User not found" });
  }
  users.delete(req.params.id);
  res.status(204).end();
});

// ---------------------------------------------------------------------------
// Global error handler
// ---------------------------------------------------------------------------

app.use((err, _req, res, _next) => {
  console.error(JSON.stringify({ level: "error", message: err.message, stack: err.stack }));
  res.status(500).json({ error: "Internal server error" });
});

// ---------------------------------------------------------------------------
// Start server (skip when imported for testing)
// ---------------------------------------------------------------------------

if (require.main === module) {
  app.listen(PORT, () => {
    console.log(
      JSON.stringify({
        level: "info",
        message: `Server started on port ${PORT}`,
        port: PORT,
        env: process.env.NODE_ENV || "development",
      })
    );
  });
}

module.exports = app;
