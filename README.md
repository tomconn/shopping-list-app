# Shopping List Web Application

A simple web application for managing a shopping list. Users can add items with quantities, view the current list, and delete items. The application uses a Go backend, a PostgreSQL database for persistence, and a plain HTML/CSS/JavaScript frontend, all containerized using Docker Compose.

## Features

*   Add food items and their quantities to the list.
*   Display the current shopping list items.
*   Delete items from the list.
*   Data persistence using a PostgreSQL database.
*   Containerized deployment using Docker Compose (Frontend, Backend, Database).
*   Backend API using Go (Golang 1.23).
*   Database connection pooling for efficiency.
*   Automatic database schema creation if the table doesn't exist.
*   Basic input sanitization (parameterized queries against SQL injection, HTML escaping against XSS on display).
*   CORS handling via Nginx proxy.

## Technology Stack

*   **Frontend:** HTML5, CSS3, Vanilla JavaScript
*   **Backend:** Go 1.24
    *   `net/http`: Web server
    *   `encoding/json`: JSON handling
    *   `github.com/jackc/pgx/v5/pgxpool`: PostgreSQL driver & connection pooling
    *   `github.com/joho/godotenv`: (Optional, for local `.env` loading)
*   **Database:** PostgreSQL 16
*   **Web Server/Proxy:** Nginx (for serving frontend & proxying API with CORS)
*   **Containerization:** Docker, Docker Compose

## Directory Structure

```text
shopping-list-app/
├── backend/
│   ├── Dockerfile        # Builds the Go backend container image
│   ├── go.mod            # Go module definition
│   ├── go.sum            # Go module checksums
│   └── main.go           # Go backend application code
├── frontend/
│   ├── index.html        # Main HTML page structure
│   ├── script.js         # Frontend JavaScript logic (API calls, DOM manipulation)
│   └── style.css         # CSS styles for the frontend
├── nginx/
│   └── nginx.conf        # Nginx configuration (serves frontend, proxies backend)
├── docker-compose.yml    # Defines and configures the multi-container application
└── README.md             # This documentation file
```

## Prerequisites

Before you begin, ensure you have the following installed:

*   [Docker Engine](https://docs.docker.com/engine/install/)
*   [Docker Compose](https://docs.docker.com/compose/install/) (Usually included with Docker Desktop)

## Setup and Running

1.  **Clone or Download:** Obtain the project files (e.g., clone the repository or download and unzip the provided archive).
2.  **Navigate:** Open a terminal or command prompt and navigate into the root directory of the project (`shopping-list-app/`).
3.  **Build and Run Containers:** Execute the following command:
    ```bash
    docker-compose up --build
    ```
    *   `--build` ensures that the Go backend image is built based on the `backend/Dockerfile`.
    *   This command will:
        *   Build the backend Go image.
        *   Pull the PostgreSQL and Nginx images if they aren't already present.
        *   Create and start containers for the database, backend, and frontend.
        *   Set up the network and volumes as defined in `docker-compose.yml`.
    *   You will see logs from all three containers. Wait for the database health check to pass and the backend/frontend to indicate they are running.
4.  **Access the Application:** Open your web browser and navigate to:
    `http://localhost:8081`
    *(Note: Port `8081` is mapped to the Nginx container's port `80` in `docker-compose.yml`)*
5.  **Interact:** Use the web interface to add and delete shopping list items. Data will be stored in the PostgreSQL container and will persist across container restarts (but not if the volume is explicitly removed).
6.  **Stop the Application:**
    *   Press `Ctrl+C` in the terminal where `docker-compose up` is running.
    *   To stop and remove the containers, networks, and volumes (including deleting all stored shopping list data), run:
        ```bash
        docker-compose down -v
        ```
    *   To stop and remove only the containers and networks (preserving the data volume), run:
        ```bash
        docker-compose down
        ```

## Code Documentation Overview

### Frontend (`frontend/`)

*   **`index.html`**: Defines the basic structure of the web page, including the heading, the form for adding items, and the unordered list (`<ul>`) where items will be displayed. Links to `style.css` and `script.js`.
*   **`style.css`**: Contains CSS rules for styling the HTML elements, providing layout, colors, and basic responsiveness.
*   **`script.js`**: Handles all client-side interactivity:
    *   Fetches the current list from the backend API (`/api/items`) on page load.
    *   Renders the fetched items into the list, creating `<li>` elements dynamically. Includes delete buttons for each item.
    *   Adds an event listener to the form to capture submissions. On submit, it sends a `POST` request to `/api/items` with the new item's name and quantity in JSON format.
    *   Adds event listeners to delete buttons. On click, it sends a `DELETE` request to `/api/items/{id}`.
    *   Refreshes the list after adding or deleting items.
    *   Includes a basic `escapeHtml` function to prevent XSS by converting special HTML characters to entities before displaying user-provided item names and quantities.

### Backend (`backend/`)

*   **`main.go`**: The core Go application:
    *   **Configuration**: Defines `DBConfig` struct and retrieves database connection details from environment variables (set in `docker-compose.yml`). Uses `getenv` helper for defaults.
    *   **Database Connection**: `connectDB` function initializes a `pgxpool` connection pool to PostgreSQL, configuring pool settings and pinging the DB to ensure connectivity.
    *   **Schema Creation**: `createSchemaIfNotExists` runs a `CREATE TABLE IF NOT EXISTS` SQL command to ensure the `items` table exists when the application starts.
    *   **Database Operations**: `getItems`, `addItem`, `deleteItem` functions handle interactions with the database. They use `context.Context` for request scoping and timeouts. **Crucially, `addItem` and `deleteItem` use parameterized queries (`$1`, `$2`) provided by `pgx` to prevent SQL injection vulnerabilities.**
    *   **HTTP Handlers**:
        *   `itemsHandler`: Routes `GET` and `POST` requests for the `/items` endpoint.
        *   `itemDetailHandler`: Routes `DELETE` requests for `/items/{id}`.
        *   Specific handlers (`getItemsHandler`, `addItemHandler`, `deleteItemHandler`): Contain the logic for handling requests, calling the appropriate database functions, handling request body decoding (with size limits and field restrictions), validating input (basic checks), setting response headers, and encoding JSON responses.
    *   **Server Setup**: Sets up an `http.ServeMux` router, registers handlers for API paths (`/items`, `/items/`), and starts the HTTP server using `http.ListenAndServe`. Includes basic server timeouts.
*   **`Dockerfile`**: Multi-stage Docker build:
    *   Stage 1 (`builder`): Uses a Go base image, copies source code, downloads dependencies, and builds a statically linked Go executable (`-ldflags="-w -s"` reduces size, `CGO_ENABLED=0` ensures static linking).
    *   Stage 2: Uses a minimal `alpine` base image, installs `ca-certificates` (important for potential HTTPS/TLS connections), copies only the compiled binary from the builder stage, exposes the application port (`8080`), and sets the `CMD` to run the executable.
*   **`go.mod`, `go.sum`**: Manage Go module dependencies.

### Nginx (`nginx/`)

*   **`nginx.conf`**: Configures the Nginx server running in the `frontend` service:
    *   Listens on port `80` inside the container.
    *   Serves static files (HTML, CSS, JS) from `/usr/share/nginx/html` (where the `frontend` volume is mounted).
    *   Defines a `location /api/` block:
        *   **Proxies** requests starting with `/api/` to the backend Go service (`http://backend:8080/`). `backend` is the service name resolved by Docker's internal DNS.
        *   **Adds CORS Headers** (`Access-Control-Allow-Origin`, `Access-Control-Allow-Methods`, etc.) to responses from the `/api/` location. This allows the JavaScript running on `localhost:8081` (served by Nginx) to make requests to the API (also served via Nginx proxy). The `'*'` allows any origin; for production, this should be restricted to the specific frontend domain.
        *   Handles CORS preflight `OPTIONS` requests.
        *   Sets standard proxy headers (`Host`, `X-Real-IP`, etc.).

### Docker Compose (`docker-compose.yml`)

*   Defines the three services (`backend`, `frontend`, `db`).
*   Specifies how to build (`backend`) or which image to use (`frontend`, `db`).
*   Sets up environment variables, particularly for database credentials and connection info, ensuring the backend can connect to the `db` service.
*   Defines port mappings (e.g., mapping host port `8081` to Nginx container port `80`).
*   Mounts volumes:
    *   Binds the local `frontend` code into the Nginx container.
    *   Binds the local `nginx.conf` into the Nginx container.
    *   Creates a named volume (`postgres_data`) to persist PostgreSQL data.
*   Sets up a custom network (`app-network`) allowing services to communicate via service names.
*   Defines `depends_on` to control startup order (e.g., backend waits for db).
*   Includes a `healthcheck` for the `db` service so the `backend` can wait until PostgreSQL is actually ready to accept connections.

## Original Prompt

> create a todo list, for a shopping list, web application.
> Follow the following instructions step by step.
> Seperate the css and the html in different files.
> Add a food item and quantity inputs and allow it to be saved.
> Display a list of foods item and quantity already stored.
> Allow deletion from the list.
> Save to a backend service written in golang 1.23, writing the food item and quantity information to a postgresql 16 db.
> Ensure all go libraries are compatible with goland 1.23.
> Ensure the inputs are parsed to prevent sql injection and xss, using the best golang library available for go 1.23.
> Ensure connection pooling is used for efficient connections to the postgresql 16 db.
> Allow schema creation, if and only, the database table does not already exist for postgresql 16.
> Allow the application to restart with an existing schema and data. Ensure all dependencies and modules are imported.
> Ensure the frontend can connect to the backend with an CORS Access-Control-Allow-Origin in the nginx.conf
> Create the docker compose file with the frontend, the backend golang app and the postgresql DB.
> Validate that the dockerfile for the backend can run correctly.
> Ensure the frontend can connect to the backend when both container running unsing docker compose.
> Ensure the frontend url is accessible from a outside of docker container
> Package the directory structure with code into a zip that I can downloaded.
>
> *Self-Correction during generation:* `escapeHtml` function in `script.js` was corrected to use `&quot;` for double quotes.
> *User Follow-up Request:* Changed `<h1>` text in `index.html` from "My Shopping List" to "Shopping List".

## AI Generation Note

This application code, structure, Docker configuration, and documentation were primarily generated by Google AI (Gemini Pro, accessed via the relevant interface/platform at the time of generation), based on the prompt and subsequent corrections/requests provided above. Manual review and minor adjustments may have been applied.

## Potential Improvements / Future Work

*   **Input Validation:** Implement more robust validation on both frontend (immediate feedback) and backend (security). Check for empty strings, potentially data types or formats for quantity.
*   **Error Handling:** Provide more specific user feedback on errors (e.g., "Item could not be saved," "Network error").
*   **Edit Functionality:** Allow users to edit existing items (name or quantity).
*   **Mark as Bought:** Add checkboxes or styling to mark items as completed/bought without deleting them.
*   **User Authentication:** Implement user accounts to allow for private, separate lists per user.
*   **Testing:** Add unit tests for backend logic and potentially integration tests for the API endpoints.
*   **Security Hardening:** Use secrets management (like Docker Secrets or HashiCorp Vault) for database credentials instead of environment variables in production. Configure HTTPS for Nginx.
*   **UI/UX Enhancements:** Improve the user interface, perhaps using a frontend framework or library for more complex interactions.
```
