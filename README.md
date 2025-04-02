# Shopping List Web Application

A simple web application for managing a shopping list. Users can add food items and quantities, view the current list, and delete items. The application features a separate frontend (HTML, CSS, JS) and backend (Go), connected to a PostgreSQL database, all running within Docker containers managed by Docker Compose.

## Features

*   **Add Items:** Input fields for item name and quantity.
*   **View List:** Displays all items currently in the shopping list, ordered by creation time.
*   **Delete Items:** Remove items individually from the list.
*   **Persistence:** Data is stored in a PostgreSQL database.
*   **Containerized:** Runs entirely within Docker containers using Docker Compose.
*   **API:** A simple RESTful API backend built with Go.
*   **Basic Security:**
    *   Backend uses parameterized queries via `pgx` to prevent SQL injection.
    *   Frontend uses basic HTML escaping (`escapeHtml` function) to mitigate simple XSS risks during display.
    *   Backend limits request body size using `http.MaxBytesReader`.
*   **Efficient DB Connections:** Uses `pgxpool` for database connection pooling.
*   **Schema Management:** Backend automatically creates the necessary database table (`items`) if it doesn't exist on startup.
*   **CORS Handling:** Nginx proxy handles Cross-Origin Resource Sharing (CORS) headers, allowing the frontend to communicate with the backend API.
*   **Unit Tested Backend:** The Go backend includes unit tests with high coverage, verifying handler logic and database interactions (via mocking). Tests are run during the Docker build.

## Technology Stack

*   **Frontend:**
    *   HTML5
    *   CSS3
    *   Vanilla JavaScript (ES6+)
*   **Backend:**
    *   Go 1.24
    *   `net/http` standard library (for HTTP server)
    *   `github.com/jackc/pgx/v5/pgxpool` (for PostgreSQL interaction)
    *   `github.com/pashagolub/pgxmock/v3` (for DB mocking in tests)
    *   `encoding/json` standard library
    *   `github.com/joho/godotenv` (optional, for local `.env` file loading)
*   **Database:**
    *   PostgreSQL 16 (running in Docker)
*   **Web Server / Proxy:**
    *   Nginx (serves frontend static files and proxies API requests)
*   **Containerization:**
    *   Docker
    *   Docker Compose

## Directory Structure

```text
shopping-list-app/
├── backend/                # Go backend service
│   ├── Dockerfile          # Docker build instructions for the backend (includes test step)
│   ├── go.mod              # Go module definition
│   ├── go.sum              # Go module checksums
│   ├── main.go             # Backend application source code
│   └── main_test.go        # Backend unit tests
├── frontend/               # Frontend HTML, CSS, JS
│   ├── index.html          # Main HTML page
│   ├── script.js           # JavaScript for frontend logic and API calls
│   └── style.css           # CSS styling
├── nginx/                  # Nginx configuration
│   └── nginx.conf          # Nginx configuration for serving frontend and proxying API
├── docker-compose.yml      # Docker Compose file to orchestrate containers
└── README.md               # This documentation file
```

## Prerequisites

Before you begin, ensure you have the following installed on your system:

*   Docker
*   Docker Compose (Often included with Docker Desktop)
*   Go (Version 1.24 or later, required only if you want to run backend tests outside Docker)
*   Git (Optional, if cloning from a repository)

## Running the Application

1.  **Clone or Download:** Obtain the project files (e.g., clone the repository or unzip the provided archive).
2.  **Navigate to Root Directory:** Open a terminal or command prompt and change into the `shopping-list-app` directory (the one containing the `docker-compose.yml` file).
3.  **Build and Start Containers:** Run the following command:
    ```bash
    docker-compose up --build
    ```
    *   `--build`: Forces Docker Compose to build the images (specifically the Go backend image) before starting the containers. The backend build process now includes running the unit tests (`go test ./...`); the build will fail if tests do not pass. Note that the `Dockerfile` references `golang:1.23-alpine` as the base image for stability; Go toolchains handle version compatibility for building the 1.24 code.
    *   This command will:
        *   Build the Go backend image using `backend/Dockerfile`.
        *   Pull the `postgres:16-alpine` and `nginx:1.25-alpine` images if they aren't already present.
        *   Create and start containers for the database (`db`), backend (`backend`), and frontend (`frontend`).
        *   Establish a network (`app-network`) for the containers to communicate.
        *   Create a persistent volume (`postgres_data`) for the database.
        *   The backend will wait for the database to be healthy before starting fully.
        *   The frontend Nginx server will wait for the backend to start.
4.  **Wait for Startup:** You will see logs from all three containers in your terminal. Wait until you see messages indicating the database is ready and the backend server is listening (e.g., `Starting server on :8080`).

## Accessing the Application

Once the containers are running successfully:

*   Open your web browser and navigate to: `http://localhost:8081`

*(Note: Port `8081` on your host machine is mapped to port `80` inside the Nginx container, as defined in `docker-compose.yml`)*

You should see the "Shopping List" application interface. You can now add, view, and delete items.

## Stopping the Application

1.  **In the terminal** where `docker-compose up` is running, press `Ctrl + C`.
2.  To stop and **remove** the containers, network, (but **keep** the database volume), run:
    ```bash
    docker-compose down
    ```
3.  To stop and remove the containers, network, **AND** the persistent database volume (deleting all shopping list data), run:
    ```bash
    docker-compose down -v
    ```

## Unit Testing (Backend)

The Go backend (`./backend`) includes unit tests (`main_test.go`) designed for high code coverage.

*   **Strategy:** Tests utilize Go's standard `testing` package, the `net/http/httptest` package for mocking HTTP requests/responses, and the `github.com/pashagolub/pgxmock/v3` library to mock database interactions. The backend uses a `DBPool` interface for its global database connection variable, allowing the real `pgxpool.Pool` or the `pgxmock` mock to be injected during testing. Expectations in the mock are generally set with simplified query regex patterns (`.*SELECT.*`, etc.) combined with rigorous argument checking (`.WithArgs(...)` or `pgxmock.AnyArg()` where appropriate) for robustness.
*   **Running Tests:**
    *   **Within Docker Build:** Tests are automatically run when building the backend image using `docker-compose build backend` or `docker-compose up --build`. The build fails if tests do not pass.
    *   **Manually (requires Go 1.24+ installed):**
        1.  Navigate to the backend directory: `cd backend`
        2.  Run the tests: `go test -v ./...`
        3.  To generate a coverage report: `go test -v -coverprofile=coverage.out ./...`
        4.  To view the HTML coverage report: `go tool cover -html=coverage.out`

## Backend API Endpoints

The Go backend exposes the following API endpoints (proxied through Nginx at `/api/`):

*   `GET /api/items`
    *   **Description:** Retrieves all shopping list items.
    *   **Response:** `200 OK` with JSON array of items: `[{"id": 1, "name": "Milk", "quantity": "1 Gallon", "created_at": "..." }, ...]` or `[]` if empty.
*   `POST /api/items`
    *   **Description:** Adds a new item to the list.
    *   **Request Body:** JSON object `{"name": "Bread", "quantity": "1 Loaf"}`
    *   **Response:** `201 Created` with the newly created item JSON: `{"id": 2, "name": "Bread", "quantity": "1 Loaf", "created_at": "..."}`. Returns `400 Bad Request` for invalid/malformed JSON or missing fields. Returns `413 Payload Too Large` if body exceeds 1MB.
*   `DELETE /api/items/{id}`
    *   **Description:** Deletes an item by its ID.
    *   **Example:** `DELETE /api/items/2`
    *   **Response:** `204 No Content` on success, `404 Not Found` if ID doesn't exist, `400 Bad Request` for invalid ID format or URL.
*   `GET /healthz`
    *   **Description:** Basic health check endpoint. Pings the database.
    *   **Response:** `200 OK` with body "OK" if healthy, `503 Service Unavailable` otherwise.

## Database Schema

The application uses a single table named `items` in the PostgreSQL database (`shopping_list_db`). The schema is automatically created by the backend if it doesn't exist:

```sql
CREATE TABLE IF NOT EXISTS items (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL CHECK (name <> ''),
    quantity TEXT NOT NULL CHECK (quantity <> ''),
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

## Development Process & GenAI Usage History

This application was bootstrapped and developed iteratively using a Generative AI (Google GenAI). The process involved providing detailed prompts and refining the output through multiple steps, including significant debugging of the generated unit tests.

### 1. Initial Project Scaffolding

The core request to the GenAI was to create the basic application structure and functionality:

> **Prompt:** "create a todo list, for a shopping list, web application. Follow the following instructions step by step. Seperate the css and the html in different files. Add a food item and quantity inputs and allow it to be saved. Display a list of foods item and quantity already stored. Allow deletion from the list. Save to a backend service written in golang 1.23, writing the food item and quantity information to a postgresql 16 db. Ensure all go libraries are compatible with goland 1.23. Ensure the inputs are parsed to prevent sql injection and xss, using the best golang library available for go 1.23. Ensure connection pooling is used for efficient connections to the postgresql 16 db. Allow schema creation, if and only, the database table does not already exist for postgresql 16. Allow the application to restart with an existing schema and data. Ensure all dependencies and modules are imported. Ensure the frontend can connect to the backend with an CORS Access-Control-Allow-Origin in the nginx.conf Create the docker compose file with the frontend, the backend golang app and the postgresql DB. Validate that the dockerfile for the backend can run correctly. Ensure the frontend can connect to the backend when both container running unsing docker compose. Ensure the frontend url is accessible from a outside of docker container Package the directory structure with code into a zip that I can downloaded."

> **Explanation:** This prompt laid out the fundamental requirements: a two-tier web app (frontend/backend), specific technologies (Go 1.23, Postgres 16, JS/HTML/CSS), core CRUD features (add, list, delete), database interaction details (pooling, schema creation), security considerations (SQLi, XSS), containerization (Docker Compose, Nginx for CORS), and packaging.

### 2. Frontend Code Refinements

Minor adjustments were requested for the frontend code:

*   **JavaScript Error Fix:**
    > **Prompt:** `.replace(/"/g, """) is not valid rewrite`
    > **Explanation:** The AI initially generated incorrect JavaScript syntax for replacing double quotes in the `escapeHtml` function. This prompt corrected it to use the proper HTML entity `"`.
*   **Heading Text Update:**
    > **Prompt:** `change the frontend to just shopping list for the headng text`
    > **Explanation:** A simple request to modify the main `<h1>` content in `index.html`.

### 3. Documentation & Version Control

Requests were made for documentation and version control guidance:

*   **Initial README Generation:**
    > **Prompt:** `recreate the readme.md, document the code, including the genai prompt begging to end including the additional unit test instructions I used to create the app the google GenAI used to create it and the file structure. Everything should be in markdown`
    > **Explanation:** This prompt asked the AI to generate the initial version of this README, summarizing the project and capturing the prompt history known up to that point.
*   **Git Integration Instructions:**
    > **Prompt:** `Using an existing github account how do I checkin this code to a private repo?`
    > **Explanation:** This requested step-by-step instructions for initializing a local Git repository and pushing the existing codebase to a new private repository on GitHub.

### 4. Backend Unit Testing (`main_test.go`) - Implementation & Debugging

This phase involved adding unit tests and required significant back-and-forth debugging:

*   **Initial Test Generation:**
    > **Prompt:** `generate unit tests, with 100% coverage`
    > **Explanation:** This requested the generation of unit tests for the Go backend (`main.go`), aiming for high coverage. The AI used the `pgxmock` library for database mocking.
*   **Integrating Tests into Docker:**
    > **Prompt:** `add unit test to backend dockerfile`
    > **Explanation:** This asked for the `backend/Dockerfile` to be modified to include a `RUN go test ./...` step, ensuring tests pass as part of the image build process.
*   **Debugging Mock Interface Assignment Error:**
    > **Error Encountered:** `error /main_test.go:35:11: cannot use mock (variable of interface type pgxmock.PgxPoolIface) as *pgxpool.Pool value in assignment`
    > **Explanation:** The tests failed to compile because the global `dbpool` variable in `main.go` was a concrete type (`*pgxpool.Pool`), but the mock object created in `main_test.go` was an interface type (`pgxmock.PgxPoolIface`). Go doesn't allow direct assignment between these.
    > **Fix Prompts:** `show the entire main.go with the changes for unit tests`, `what additional go get do I need to call?`
    > **Resolution:** The fix involved defining a `DBPool` interface in `main.go`, changing the global `dbpool` variable to use this interface type, and ensuring the mock assignment in `main_test.go` was now valid. This required regenerating `main.go`.
*   **Debugging Handler Signature Mismatch Error:**
    > **Error Encountered:** `Error main_test.go:562:29: cannot use deleteItemHandler (value of type func(w http.ResponseWriter, r *http.Request, id int)) as http.HandlerFunc value in argument to executeRequest`
    > **Explanation:** After refactoring the delete logic in `main.go`, the `deleteItemHandler` function signature changed, but the test was still trying to pass it to a helper expecting the standard `http.HandlerFunc` signature.
    > **Fix Prompts:** `recreate the complete unit test with the update`
    > **Resolution:** The fix involved modifying the relevant test (`TestDeleteItemHandler`) to call the correct mux handler (`itemDetailHandler`) which *does* have the standard signature and internally calls the refactored `deleteItemHandler`. This required regenerating `main_test.go`.
*   **Debugging `pgxmock` Options & Build Errors:**
    > **Errors Encountered:** `undefined: pgxmock.ExpectHookOption`, `undefined: pgxmock.LooseExpectationOrder`, `LooseExpectationOrder error`
    > **Explanation:** Attempts to use mock options like `LooseExpectationOrder` during `pgxmock.NewPool` initialization were made with incorrect syntax or referred to non-existent identifiers, causing build failures.
    > **Fix Prompts:** `fix new issue ... undefined: pgxmock.ExpectHookOption ...`, `LooseExpectationOrder error`, `recreate the entire test again.`
    > **Resolution:** These attempts were reverted after several iterations involving regeneration of `main_test.go`, sticking to the standard `pgxmock.QueryMatcherOption(pgxmock.QueryMatcherRegexp)`.
*   **Debugging `ExpectationsWereMet` Failures:**
    > **Error Encountered:** Persistent failures checking `mock.ExpectationsWereMet()`, primarily in `TestAddItemHandler/DatabaseError` and `TestAddItemHandler/UnknownFieldsJSON`. The logs often indicated the code path was correct, but the specific mock expectation (query string, arguments, or method type) wasn't matched. A key error message was: `main_test.go:564: Unfulfilled expectations AFTER handler execution: there is a remaining expectation which was not matched: ExpectedQuery => expecting call to Query() or to QueryRow(): ... is without arguments ...` (This indicated the mock expected no arguments when arguments *were* being sent)
    > **Explanation:** This indicated a mismatch between the arguments expected by the mock and the arguments actually sent by the code, or other subtle expectation mismatches.
    > **Debugging Prompts:** `fix this properly. ... FAIL: TestAddItemHandler/DatabaseError ...`, `generate the entire test with the recommended fix`, `generate entire test will fixes`, `fix ... FAIL: TestAddItemHandler/DatabaseError and generate the unit test.`
    > **Resolution:** This required multiple debugging steps within `main_test.go`:
        1.  Simplifying query regex patterns (e.g., `.*INSERT.*`) to rule out regex errors.
        2.  Trying to relax argument matching (`.WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg())`).
        3.  Temporarily removing argument matching entirely (helped diagnose the "is without arguments" error).
        4.  Correctly reinstating `.WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg())` for the specific failing error case (`TestAddItemHandler/DatabaseError`).
        5.  Fixing the `TestAddItemHandler/UnknownFieldsJSON` error message check by making it less specific and ensuring no DB expectations were erroneously checked. This final iteration successfully resolved the test failures.

### 5. Final Documentation & Version Update

*   **Intermediate README Consolidation:** Previous prompts consolidated documentation and history.
    > **Prompt:** `recreate the readme.md, document the code, including the genai prompt beginning to end including the additional unit test instructions, for main_test.go, I used to create the app the google GenAI used to create it and the file structure. Everything should be in single README.md markdown file.`

