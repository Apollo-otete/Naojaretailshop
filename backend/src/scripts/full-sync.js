const { exec } = require('child_process');
const chokidar = require('chokidar');
const fs = require('fs');
const path = require('path');

const DB_NAME = 'Naojaretail';
const DB_USER = 'postgres';
const DB_PASSWORD = 'bagy9541';
const SCHEMA_FILE = path.join(__dirname, '../migrations/naoja_schema.sql');

console.log('🔄 Two-Way Sync Started!');
console.log(`📁 Watching: ${SCHEMA_FILE}`);
console.log(`📊 Database: ${DB_NAME}`);

// 1. Watch for changes in the SQL file (VS Code → pgAdmin)
const watcher = chokidar.watch(SCHEMA_FILE, {
    persistent: true,
    awaitWriteFinish: { stabilityThreshold: 1000, pollInterval: 100 }
});

watcher.on('change', (filePath) => {
    console.log(`📝 VS Code change detected!`);
    console.log(`🔄 Syncing to PostgreSQL...`);

    const command = `PGPASSWORD=${DB_PASSWORD} psql -d ${DB_NAME} -U ${DB_USER} -h localhost -f "${filePath}"`;

    exec(command, (error, stdout, stderr) => {
        if (error) {
            console.error(`❌ Error: ${error.message}`);
            return;
        }
        console.log(`✅ Changes applied to PostgreSQL!`);
        console.log(`📊 ${stdout || 'Success!'}`);
    });
});

// 2. Function to export from PostgreSQL to file (pgAdmin → VS Code)
function exportSchema() {
    console.log(`🔄 Exporting schema from PostgreSQL...`);

    const command = `PGPASSWORD=${DB_PASSWORD} pg_dump -s -h localhost -U ${DB_USER} -d ${DB_NAME} > ${SCHEMA_FILE}`;

    exec(command, (error, stdout, stderr) => {
        if (error) {
            console.error(`❌ Export error: ${error.message}`);
            return;
        }
        // FIXED: This line was the main error
        console.log(`✅ Schema exported to VS Code!`);
        console.log(`📁 Updated: ${SCHEMA_FILE}`);
    });
}

// 3. Ask user for manual export option
console.log('💡 To export schema from pgAdmin to VS Code, run: npm run sync:from-db');
console.log('💡 To watch for changes, this script is already running!');

// Export when the script starts (initial sync)
exportSchema();

// Export every 30 seconds as a fallback (optional)
setInterval(exportSchema, 30000);

console.log('✅ Two-way sync is active!');
console.log(`📂 SQL file: ${SCHEMA_FILE}`);
console.log(`🗄️  Database: ${DB_NAME}`);