// Global state
let files = [];
let charts = {};
let timeFrom = null;
let timeTo = null;

let appSettings = {
    smoothLines: true,
    showPoints: false,
    fillArea: true
};

let activeFileId = null;

// Initialize app
document.addEventListener('DOMContentLoaded', () => {
    loadSettings();
    setupEventListeners();
    loadVersion();
    loadFiles();
});

function loadSettings() {
    const saved = localStorage.getItem('unostat_settings');
    if (saved) {
        try {
            appSettings = { ...appSettings, ...JSON.parse(saved) };
        } catch (e) { console.error("Failed to parse settings", e); }
    }

    const smoothEl = document.getElementById('settingSmoothLines');
    const pointsEl = document.getElementById('settingShowPoints');
    const fillEl = document.getElementById('settingFillArea');

    if (smoothEl) smoothEl.checked = appSettings.smoothLines;
    if (pointsEl) pointsEl.checked = appSettings.showPoints;
    if (fillEl) fillEl.checked = appSettings.fillArea;
}

function setupEventListeners() {
    const fileInput = document.getElementById('fileInput');
    const uploadBtn = document.getElementById('uploadBtn');
    const dropzone = document.getElementById('uploadPrompt');

    // Sidebar Upload
    if (uploadBtn) {
        uploadBtn.addEventListener('click', () => fileInput.click());
    }

    // Main Dropzone
    if (dropzone) {
        dropzone.addEventListener('dragover', (e) => {
            e.preventDefault();
            dropzone.classList.add('dragover');
        });
        dropzone.addEventListener('dragleave', () => {
            dropzone.classList.remove('dragover');
        });
        dropzone.addEventListener('drop', (e) => {
            e.preventDefault();
            dropzone.classList.remove('dragover');
            handleFileSelect({ target: { files: e.dataTransfer.files } });
        });
    }

    fileInput.addEventListener('change', handleFileSelect);
    document.getElementById('applyFilters').addEventListener('click', applyFilters);
    document.getElementById('resetFilters').addEventListener('click', resetFilters);
    document.getElementById('exportAllBtn').addEventListener('click', exportAllCharts);

    // Navigation Listeners
    document.getElementById('navDashboard').addEventListener('click', (e) => {
        e.preventDefault();
        switchView('dashboard');
    });

    document.getElementById('navSettings').addEventListener('click', (e) => {
        e.preventDefault();
        switchView('settings');
    });

    // Settings Controls
    document.getElementById('settingSmoothLines').addEventListener('change', (e) => updateSetting('smoothLines', e.target.checked));
    document.getElementById('settingShowPoints').addEventListener('change', (e) => updateSetting('showPoints', e.target.checked));
    document.getElementById('settingFillArea').addEventListener('change', (e) => updateSetting('fillArea', e.target.checked));

    // Clear data buttons
    const clearBtn = document.getElementById('clearAllDataBtn'); // Settings page
    if (clearBtn) clearBtn.addEventListener('click', clearAllData);

    const sidebarClear = document.getElementById('sidebarClearAllBtn'); // Sidebar
    if (sidebarClear) sidebarClear.addEventListener('click', (e) => {
        e.stopPropagation(); // prevent card click if any
        clearAllData();
    });
}

async function loadVersion() {
    try {
        const response = await fetch('/api/version');
        if (!response.ok) {
            throw new Error('Failed to fetch version');
        }
        const data = await response.json();

        // Format version string
        const versionText = `UnoStat ${data.version}`;

        // Update sidebar version
        const sidebarVersion = document.getElementById('sidebarVersion');
        if (sidebarVersion) {
            sidebarVersion.textContent = versionText;
        }

        // Update settings page version
        const settingsVersion = document.getElementById('settingsVersion');
        if (settingsVersion) {
            settingsVersion.textContent = data.version;
        }
    } catch (error) {
        console.error('Failed to load version:', error);
        // Set fallback text
        const sidebarVersion = document.getElementById('sidebarVersion');
        if (sidebarVersion) sidebarVersion.textContent = 'UnoStat';

        const settingsVersion = document.getElementById('settingsVersion');
        if (settingsVersion) settingsVersion.textContent = 'Unknown';
    }
}

function switchView(viewName) {
    // Nav Items
    const navDash = document.getElementById('navDashboard');
    const navSettings = document.getElementById('navSettings');

    // Views
    const viewDash = document.getElementById('dashboardView');
    const viewSettings = document.getElementById('settingsView');

    const pageTitle = document.getElementById('pageTitle');
    const pageSubtitle = document.getElementById('pageSubtitle');
    const toolbar = document.getElementById('dashboardToolbar');

    if (viewName === 'dashboard') {
        navDash.classList.add('active');
        navSettings.classList.remove('active');

        viewDash.classList.remove('hidden');
        viewSettings.classList.add('hidden');

        pageTitle.innerHTML = '<i class="fa-solid fa-chart-simple"></i> Performance Charts';

        const activeFile = files.find(f => f.id === activeFileId);
        if (activeFile && pageSubtitle) {
            pageSubtitle.innerHTML = `<i class="fa-solid fa-file-csv"></i> ${escapeHtml(activeFile.name)}`;
        } else if (pageSubtitle) {
            pageSubtitle.innerHTML = '';
        }

        if (toolbar) toolbar.style.visibility = 'visible';
    } else if (viewName === 'settings') {
        navDash.classList.remove('active');
        navSettings.classList.add('active');

        viewDash.classList.add('hidden');
        viewSettings.classList.remove('hidden');

        pageTitle.innerHTML = '<i class="fa-solid fa-gear"></i> System Settings';
        if (pageSubtitle) pageSubtitle.innerHTML = '';

        if (toolbar) toolbar.style.visibility = 'hidden';
    }
}


async function handleFileSelect(e) {
    const selectedFiles = Array.from(e.target.files);

    if (selectedFiles.length === 0) return;

    showToast(`Uploading ${selectedFiles.length} file(s)...`, 'info');

    let successCount = 0;
    for (const file of selectedFiles) {
        if (file.name.endsWith('.csv')) {
            const success = await uploadFile(file);
            if (success) successCount++;
        } else {
            showToast(`Skipped ${file.name}: Not a CSV file`, 'error');
        }
    }

    e.target.value = '';
    if (successCount > 0) {
        showToast(`Successfully uploaded ${successCount} files`, 'success');
        loadFiles();
    }
}

async function uploadFile(file) {
    const formData = new FormData();
    formData.append('file', file);

    try {
        const response = await fetch('/api/files/upload', {
            method: 'POST',
            body: formData
        });

        if (!response.ok) {
            const error = await response.json();
            showToast(`Failed to upload ${file.name}: ${error.error}`, 'error');
            return false;
        }
        return true;
    } catch (error) {
        showToast(`Network error uploading ${file.name}`, 'error');
        return false;
    }
}

async function loadFiles() {
    try {
        const response = await fetch('/api/files');
        if (!response.ok) {
            const errText = await response.text();
            throw new Error(`Server returned ${response.status}: ${errText}`);
        }

        try {
            files = await response.json();
        } catch (jsonError) {
            throw new Error(`Invalid JSON: ${jsonError.message}`);
        }

        if (!Array.isArray(files)) {
            throw new Error("Response is not an array");
        }

        const countEl = document.getElementById('fileCount');
        if (countEl) countEl.textContent = files.length;

        // Auto selection logic
        if (files.length > 0) {
            if (!activeFileId || !files.find(f => f.id === activeFileId)) {
                // Determine best file to select (e.g. valid one)
                const validFile = files.find(f => f.id !== activeFileId); // Just pick first one for now or keep existing if valid
                activeFileId = validFile ? validFile.id : files[0].id;
            }
        } else {
            activeFileId = null;
        }

        renderFilesList();
        updateViewState();

        if (files.length > 0 && activeFileId) {
            await loadAllCharts();
        }
    } catch (error) {
        console.error('Failed to load files:', error);
        showToast(`Failed to load file list: ${error.message}`, 'error');
    }
}

function renderFilesList() {
    const container = document.getElementById('filesList');

    if (files.length === 0) {
        container.innerHTML = '<div class="empty-state-sm">No files loaded</div>';
        return;
    }

    container.innerHTML = files.map(file => {
        const isActive = file.id === activeFileId ? 'active' : '';
        const isLoaded = file.isLoaded;
        const loadStatus = isLoaded ? '' : '<span class="status-badge" title="Not Loaded (Click check to load)"><i class="fa-regular fa-circle"></i></span>';
        // If loaded, show check. If not, show button to load.
        // Actually simpler: if !isLoaded, show Load button. If loaded, show nothing or loaded icon.
        // Let's integrate a "Load" action.

        let actionBtn = '';
        if (!isLoaded) {
            actionBtn = `
            <button class="file-action-btn load-btn" onclick="event.stopPropagation(); loadFileContent('${file.id}')" title="Load Data">
                <i class="fa-solid fa-cloud-arrow-down"></i>
            </button>`;
        }

        return `
        <div class="file-card ${isActive} ${!isLoaded ? 'unloaded' : ''}" onclick="selectFile('${file.id}')">
            <div class="file-actions-right">
                ${actionBtn}
                <button class="file-action-btn delete-btn" onclick="event.stopPropagation(); deleteFile('${file.id}')" title="Delete file">
                    <i class="fa-solid fa-times"></i>
                </button>
            </div>
            <span class="file-name" title="${escapeHtml(file.name)}">${escapeHtml(file.name)}</span>
            <div class="file-info text-xsmall">
                ${isLoaded ?
                `<div><i class="fa-solid fa-table-cells"></i> ${file.rowCount.toLocaleString()} rows</div>` :
                `<div><i class="fa-regular fa-circle"></i> Not Loaded</div>`
            }
                <div><i class="fa-regular fa-clock"></i> ${formatTimeShort(file.minTime)} - ${formatTimeShort(file.maxTime)}</div>
            </div>
        </div>
    `}).join('');
}

async function loadFileContent(fileId) {
    // Show loading state for specific file card if possible, or global toast
    showToast('Loading file content...', 'info');

    // Optimistic UI update or disable button could be nice, but simple toast is okay for now.
    try {
        const response = await fetch(`/api/files/${fileId}/load`, { method: 'POST' });
        if (!response.ok) {
            const err = await response.json();
            throw new Error(err.error || 'Failed to load');
        }

        showToast('File loaded successfully', 'success');
        await loadFiles(); // Refresh list to update status

        // If this file matches active, reload charts
        if (activeFileId === fileId) {
            loadAllCharts();
        }
    } catch (error) {
        showToast(`Error loading file: ${error.message}`, 'error');
    }
}

function selectFile(fileId) {
    switchView('dashboard');
    if (activeFileId === fileId) return;

    // Check if loaded
    const file = files.find(f => f.id === fileId);
    if (file && !file.isLoaded) {
        // If clicking an unloaded file, verify if user wants to load it or just select it (empty view)
        // Current logic: Select it. User sees "No metrics".
        // Enhanced logic: Trigger load automatically?
        // Let's stick to "User clicks Load button" or "User clicks Card -> select".
        // If selected but not loaded, maybe show a big "Load Data" button in the center view?
    }

    activeFileId = fileId;
    renderFilesList(); // Re-render to update active styling

    // If not loaded, loadCheck will fail gracefully (0 metrics) or we can prompt.
    // existing loadAllCharts handles 0 metrics.
    loadAllCharts();
}

async function deleteFile(fileId) {
    if (!confirm('Are you sure you want to delete this file?')) return;

    try {
        const response = await fetch(`/api/files/${fileId}`, { method: 'DELETE' });
        if (response.ok) {
            showToast('File deleted', 'success');
            loadFiles();
        } else {
            showToast('Failed to delete file', 'error');
        }
    } catch (error) {
        showToast('Network error deleting file', 'error');
    }
}

function updateViewState() {
    const uploadPrompt = document.getElementById('uploadPrompt');
    const chartsContainer = document.getElementById('chartsContainer');
    const statsOverview = document.getElementById('statsOverview');

    // Update stats
    const totalFilesEl = document.getElementById('statsTotalFiles');
    if (totalFilesEl) totalFilesEl.innerText = files.length;

    if (files.length > 0) {
        if (uploadPrompt) uploadPrompt.classList.add('hidden');
        chartsContainer.classList.remove('hidden');
    } else {
        if (uploadPrompt) uploadPrompt.classList.remove('hidden');

        chartsContainer.classList.add('hidden');
        chartsContainer.innerHTML = '';
        charts = {};
        const totalMetricsEl = document.getElementById('statsTotalMetrics');
        if (totalMetricsEl) totalMetricsEl.innerText = '0';
    }
}

async function loadAllCharts() {
    const container = document.getElementById('chartsContainer');
    container.innerHTML = '';

    // If no active file, show nothing or empty state
    if (!activeFileId || !files.find(f => f.id === activeFileId)) {
        updateViewState(); // Make sure empty state is shown if needed
        return;
    }

    const file = files.find(f => f.id === activeFileId);

    // Update Page Subtitle with filename (Title is static)
    const pageSubtitle = document.getElementById('pageSubtitle');
    if (pageSubtitle) {
        pageSubtitle.innerHTML = `<i class="fa-solid fa-file-csv"></i> ${escapeHtml(file.name)}`;
    }

    const chartsToRender = [];
    let totalMetricsCount = 0;

    // Load Metrics for ONE file
    try {
        const response = await fetch(`/api/files/${file.id}/metrics`);
        const data = await response.json();
        const metrics = data.metrics;
        totalMetricsCount = metrics.length;

        for (const metric of metrics) {
            const chartId = `chart-${file.id}-${sanitizeId(metric)}`;

            const card = document.createElement('div');
            card.className = 'chart-card';
            // Removed file.name from <p> tag as requested
            card.innerHTML = `
                <div class="chart-header">
                    <div class="chart-title">
                        <h3>${escapeHtml(metric)}</h3>
                    </div>
                    <div class="chart-actions">
                        <button class="btn-icon" onclick="exportChart('${chartId}', '${escapeHtml(metric)}', '${escapeHtml(file.name)}')" title="Download Image">
                            <i class="fa-solid fa-download"></i>
                        </button>
                    </div>
                </div>
                <div class="chart-body">
                    <canvas id="${chartId}"></canvas>
                </div>
            `;
            container.appendChild(card);

            chartsToRender.push({ file, metric, chartId });
        }
    } catch (error) {
        console.error(`Failed to load metrics for ${file.name}:`, error);
        showToast(`Failed to load metrics for ${file.name}`, 'error');
    }

    // Update metric stats
    const totalMetricsEl = document.getElementById('statsTotalMetrics');
    if (totalMetricsEl) totalMetricsEl.innerText = totalMetricsCount;

    if (chartsToRender.length === 0) {
        container.innerHTML = '<div class="empty-state-sm" style="grid-column: 1/-1; text-align: center;">No metrics found in this file</div>';
        return;
    }

    await Promise.all(chartsToRender.map(item => renderChart(item.file, item.metric, item.chartId)));
}

async function renderChart(file, metric, chartId) {
    const canvas = document.getElementById(chartId);
    if (!canvas) return;

    const ctx = canvas.getContext('2d');

    if (charts[chartId]) {
        charts[chartId].destroy();
        delete charts[chartId];
    }

    let url = `/api/data/${file.id}/${encodeURIComponent(metric)}`;
    const params = new URLSearchParams();
    if (timeFrom) params.append('from', timeFrom);
    if (timeTo) params.append('to', timeTo);
    if (params.toString()) url += '?' + params.toString();

    try {
        const response = await fetch(url);
        const dataPoints = await response.json();

        // Calculate Stats
        const values = dataPoints.map(d => d.value).filter(v => !isNaN(v));
        let min = 0, max = 0, avg = 0;
        if (values.length > 0) {
            min = Math.min(...values);
            max = Math.max(...values);
            const sum = values.reduce((a, b) => a + b, 0);
            avg = sum / values.length;
        }



        // Chart.js Data
        const data = dataPoints.map(d => ({ x: d.timestamp, y: d.value }));
        const color = getMetricColor(metric);

        // Define Stats Plugin for Export/Canvas drawing
        const statsPlugin = {
            id: 'statsPlugin',
            afterDraw: (chart) => {
                const ctx = chart.ctx;
                ctx.save();
                ctx.font = '12px Inter';
                ctx.fillStyle = '#7d8590';
                ctx.textAlign = 'right';
                // Draw stats on top right of chart area
                const text = `Min: ${min.toFixed(2)} | Avg: ${avg.toFixed(2)} | Max: ${max.toFixed(2)}`;
                ctx.fillText(text, chart.chartArea.right, chart.chartArea.top - 10);
                ctx.restore();
            }
        };

        charts[chartId] = new Chart(ctx, {
            type: 'line',
            data: {
                datasets: [{
                    label: metric,
                    data: data,
                    borderColor: color,
                    backgroundColor: hexToRgba(color, 0.1),
                    borderWidth: 1.5,
                    fill: appSettings.fillArea,
                    tension: appSettings.smoothLines ? 0.3 : 0,
                    pointRadius: appSettings.showPoints ? 3 : 0,
                    pointHitRadius: 10,
                    pointHoverRadius: 4
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                animation: false,
                layout: {
                    padding: {
                        top: 25,
                        right: 10,
                        bottom: 10,
                        left: 10
                    }
                },
                plugins: {
                    legend: { display: false },
                    tooltip: {
                        mode: 'index',
                        intersect: false,
                        backgroundColor: '#1c2128',
                        titleColor: '#e6edf3',
                        bodyColor: '#e6edf3',
                        borderColor: '#30363d',
                        borderWidth: 1,
                        padding: 10,
                        displayColors: false,
                        callbacks: {
                            label: function (context) {
                                return `Value: ${context.parsed.y.toFixed(2)}`;
                            }
                        }
                    },
                    zoom: {
                        zoom: {
                            drag: {
                                enabled: true,
                                backgroundColor: 'rgba(59, 130, 246, 0.2)',
                                borderColor: '#3b82f6',
                                borderWidth: 1,
                                threshold: 10
                            },
                            mode: 'x',
                            onZoomComplete: function ({ chart }) {
                                const { min, max } = chart.scales.x;
                                const fromDate = new Date(min);
                                const toDate = new Date(max);

                                timeFrom = fromDate.toISOString();
                                timeTo = toDate.toISOString();

                                const formatForInput = (date) => {
                                    const offset = date.getTimezoneOffset() * 60000;
                                    return (new Date(date - offset)).toISOString().slice(0, 19);
                                };

                                document.getElementById('timeFrom').value = formatForInput(fromDate);
                                document.getElementById('timeTo').value = formatForInput(toDate);

                                showToast('Time range updated', 'info');
                                loadAllCharts();
                            }
                        }
                    }
                },
                scales: {
                    x: {
                        type: 'time',
                        time: {
                            displayFormats: {
                                millisecond: 'HH:mm:ss.SSS',
                                second: 'HH:mm:ss',
                                minute: 'HH:mm',
                                hour: 'dd/MM HH:mm'
                            },
                            tooltipFormat: 'yyyy-MM-dd HH:mm:ss'
                        },
                        grid: { color: '#30363d', tickLength: 4 },
                        ticks: { color: '#7d8590', maxRotation: 0, autoSkip: true }
                    },
                    y: {
                        beginAtZero: true,
                        grid: { color: '#262c36' },
                        ticks: { color: '#7d8590' }
                    }
                },
                interaction: {
                    mode: 'nearest',
                    axis: 'x',
                    intersect: false
                }
            },
            plugins: [statsPlugin] // Register stats plugin
        });
    } catch (e) {
        console.error("Chart render error", e);
        canvas.parentNode.innerHTML = '<div class="empty-state-sm">Failed to load data</div>';
    }
}

// Helpers

/**
 * Converts a date string (from datetime-local input) to an ISO 8601 string.
 * This effectively converts the local browser time to UTC, matching the backend's data handling.
 * Example: Input "00:00" Local (VN +7) -> Output "17:00Z" (Previous Day)
 */
function getLocalISOString(dateString) {
    if (!dateString) return null;
    const date = new Date(dateString);
    if (isNaN(date.getTime())) return null;
    return date.toISOString();
}

/**
 * Applies the time filter based on user inputs.
 */
function applyFilters() {
    const fromInput = document.getElementById('timeFrom').value;
    const toInput = document.getElementById('timeTo').value;

    timeFrom = getLocalISOString(fromInput);
    timeTo = getLocalISOString(toInput);

    // Validate range
    if (timeFrom && timeTo && timeFrom > timeTo) {
        showToast('Invalid time range: Start time must be before end time', 'error');
        return;
    }

    loadAllCharts();
    showToast('Time filter applied successfully', 'success');
}

function resetFilters() {
    document.getElementById('timeFrom').value = '';
    document.getElementById('timeTo').value = '';
    timeFrom = null;
    timeTo = null;
    loadAllCharts();
}

async function exportChart(chartId, metricName, fileName) {
    const chart = charts[chartId];
    if (!chart) return;

    showToast('Preparing download...', 'info');

    // Reuse data from existing chart
    const dataset = chart.data.datasets[0];
    // Pass raw data array directly
    const base64 = await generateChartImage(metricName, dataset.data, true);

    const link = document.createElement('a');
    const cleanMetric = sanitizeId(metricName).replace(/-/g, '_').replace(/_+/g, '_');
    link.download = `${cleanMetric}.png`;
    link.href = base64;
    link.click();
}

async function exportAllCharts() {
    if (files.length === 0) return;

    showToast('Starting batch export (All Files)...', 'info');

    try {
        const tar = new TarWriter();
        const totalFiles = files.length;
        let processedCount = 0;

        for (const file of files) {
            processedCount++;
            showToast(`Processing file ${processedCount}/${totalFiles}: ${file.name}`, 'info');

            // 1. Fetch metrics for this file
            try {
                const mRes = await fetch(`/api/files/${file.id}/metrics`);
                const mData = await mRes.json();
                const metrics = mData.metrics;

                for (const metric of metrics) {
                    // 2. Fetch data points
                    let url = `/api/data/${file.id}/${encodeURIComponent(metric)}`;
                    const params = new URLSearchParams();
                    if (timeFrom) params.append('from', timeFrom);
                    if (timeTo) params.append('to', timeTo);
                    if (params.toString()) url += '?' + params.toString();

                    const dRes = await fetch(url);
                    const dataPoints = await dRes.json();

                    if (!dataPoints || dataPoints.length === 0) continue;

                    // 3. Render to hidden canvas
                    // Convert raw response to {x,y} format
                    const data = dataPoints.map(d => ({ x: d.timestamp, y: d.value }));
                    const base64 = await generateChartImage(metric, data, true);

                    // 4. Add to TAR with folder structure: FileName/Metric.png
                    // Convert base64 to bytes
                    const binaryString = window.atob(base64.split(',')[1]);
                    const len = binaryString.length;
                    const bytes = new Uint8Array(len);
                    for (let i = 0; i < len; i++) {
                        bytes[i] = binaryString.charCodeAt(i);
                    }

                    // Clean names
                    const folderName = sanitizeId(file.name).replace(/-/g, '_').replace(/_+/g, '_');
                    const fileName = sanitizeId(metric).replace(/-/g, '_').replace(/_+/g, '_');
                    const tarPath = `${folderName}/${fileName}.png`;

                    tar.addFile(tarPath, bytes);
                }
            } catch (err) {
                console.error(`Error processing file ${file.name}`, err);
            }

            // Small delay to allow UI update
            await new Promise(resolve => setTimeout(resolve, 100));
        }

        const now = new Date();
        const timestamp = now.toISOString().replace(/[-:T.]/g, '').slice(0, 14);
        const tarName = `${timestamp}_full_dataset_export.tar`;

        tar.download(tarName);
        showToast('Batch export completed!', 'success');

    } catch (e) {
        console.error(e);
        showToast('Failed to create TAR file', 'error');
    }
}

function generateChartImage(metric, formattedData, isFormatted = false) {
    return new Promise((resolve) => {
        const canvas = document.createElement('canvas');
        canvas.width = 1200; // High resolution for export
        canvas.height = 600;
        const ctx = canvas.getContext('2d');

        // Handle both raw API data and pre-formatted chart data
        let data = formattedData;
        if (!isFormatted) {
            data = formattedData.map(d => ({ x: d.timestamp, y: d.value }));
        }

        const values = data.map(d => d.y).filter(v => !isNaN(v));
        let min = 0, max = 0, avg = 0;
        if (values.length > 0) {
            min = Math.min(...values);
            max = Math.max(...values);
            const sum = values.reduce((a, b) => a + b, 0);
            avg = sum / values.length;
        }

        const color = getMetricColor(metric);

        // Plugin to fill background color (Dark Theme #121214) AND Draw Stats
        const customPlugin = {
            id: 'customExportPlugin',
            beforeDraw: (chart) => {
                const { ctx, width, height } = chart;
                ctx.save();
                ctx.fillStyle = '#121214';
                ctx.fillRect(0, 0, width, height);
                ctx.restore();
            },
            afterDraw: (chart) => {
                const ctx = chart.ctx;
                ctx.save();
                ctx.font = 'bold 16px Inter, sans-serif';
                ctx.fillStyle = '#a1a1aa';
                ctx.textAlign = 'right';
                // Draw stats below title
                const text = `Min: ${min.toFixed(2)} | Avg: ${avg.toFixed(2)} | Max: ${max.toFixed(2)}`;
                ctx.fillText(text, chart.chartArea.right, chart.chartArea.top - 15);
                ctx.restore();
            }
        };

        const chart = new Chart(ctx, {
            type: 'line',
            data: {
                datasets: [{
                    label: metric,
                    data: data,
                    borderColor: color,
                    backgroundColor: hexToRgba(color, 0.15),
                    borderWidth: 2,
                    fill: appSettings.fillArea,
                    tension: appSettings.smoothLines ? 0.3 : 0,
                    pointRadius: appSettings.showPoints ? 3 : 0,
                    pointHitRadius: 10
                }]
            },
            options: {
                responsive: false,
                animation: false,
                devicePixelRatio: 2,
                layout: {
                    padding: { top: 50, bottom: 20, left: 20, right: 20 }
                },
                plugins: {
                    legend: { display: false },
                    title: {
                        display: true,
                        text: metric,
                        color: '#f4f4f5',
                        font: { size: 24, weight: 'bold', family: 'Inter, sans-serif' },
                        padding: { top: 10, bottom: 30 },
                        align: 'center'
                    }
                },
                scales: {
                    x: {
                        type: 'time',
                        time: {
                            displayFormats: {
                                millisecond: 'HH:mm:ss.SSS',
                                second: 'HH:mm:ss',
                                minute: 'HH:mm',
                                hour: 'dd/MM HH:mm'
                            },
                        },
                        grid: { color: '#30363d', tickLength: 8 },
                        ticks: { color: '#a1a1aa', font: { size: 14 } }
                    },
                    y: {
                        beginAtZero: true,
                        grid: { color: '#27272a' },
                        ticks: { color: '#a1a1aa', font: { size: 14 } }
                    }
                }
            },
            plugins: [customPlugin]
        });

        // setTimeout ensures the rendering frame is complete
        setTimeout(() => {
            const img = chart.toBase64Image('image/png', 1);
            chart.destroy();
            resolve(img);
        }, 100);
    });
}

// Simple TAR Writer implementation
class TarWriter {
    constructor() {
        this.files = [];
    }

    addFile(name, data) {
        this.files.push({ name, data });
    }

    download(filename) {
        let totalSize = 0;
        this.files.forEach(file => {
            totalSize += 512; // Header
            totalSize += Math.ceil(file.data.length / 512) * 512; // Content aligned to 512
        });
        totalSize += 1024; // 2 empty blocks at end

        const buffer = new Uint8Array(totalSize);
        let offset = 0;

        const writeString = (str, len, off) => {
            for (let i = 0; i < len && i < str.length; i++) {
                buffer[off + i] = str.charCodeAt(i);
            }
        };

        const writeOctal = (num, len, off) => {
            const s = num.toString(8).padStart(len - 1, '0');
            writeString(s, len - 1, off);
        };

        this.files.forEach(file => {
            const headerStart = offset;
            const data = file.data;

            // name (100)
            writeString(file.name, 100, headerStart); // Use full path for standard TAR, but max 100 chars here is tight for nested.
            // But basic UStar might handle prefix. For now let's hope names are short enough or truncate.
            // file mode (8) - 0644 for regular file
            writeString("0000644", 7, headerStart + 100);
            // uid (8)
            writeOctal(0, 8, headerStart + 108);
            // gid (8)
            writeOctal(0, 8, headerStart + 116);
            // size (12)
            writeOctal(data.length, 12, headerStart + 124);
            // mtime (12)
            writeOctal(Math.floor(Date.now() / 1000), 12, headerStart + 136);
            // chksum (8) - blank first
            writeString("        ", 8, headerStart + 148);
            // typeflag (1)
            buffer[headerStart + 156] = '0'.charCodeAt(0);
            // magic (6)
            writeString("ustar", 5, headerStart + 257);
            // version (2)
            writeString("00", 2, headerStart + 263);

            // Calculate checksum
            let sum = 0;
            for (let i = 0; i < 512; i++) sum += buffer[headerStart + i];
            writeOctal(sum, 7, headerStart + 148); // Rewrite checksum
            buffer[headerStart + 155] = 0; // Null terminator for checksum? typically space or null.
            // Standard tar sum ends with null and space usually?
            // "The checksum is calculated by taking the sum of the unsigned byte values of the header record with the eight checksum bytes taken to be ascii spaces (decimal value 32). It is stored as a six digit octal number with leading zeroes, followed by a NUL and then a space."

            // Correct checksum formatting for ustar: 6 digits, null, space.
            // writeOctal writes 7 chars + implied processing.
            // Let's stick to a simpler known working checksum write:
            const octalSum = sum.toString(8).padStart(6, '0');
            for (let i = 0; i < 6; i++) buffer[headerStart + 148 + i] = octalSum.charCodeAt(i);
            buffer[headerStart + 154] = 0;
            buffer[headerStart + 155] = 32;

            offset += 512;

            // Write Data
            buffer.set(data, offset);
            offset += Math.ceil(data.length / 512) * 512;
        });

        const blob = new Blob([buffer], { type: 'application/x-tar' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = filename;
        link.click();
        URL.revokeObjectURL(url);
    }
}

// Helpers
function getMetricColor(metric) {
    const m = metric.toLowerCase();
    if (m.includes('cpu')) return '#3b82f6'; // Blue
    if (m.includes('mem') || m.includes('ram')) return '#238636'; // Green
    if (m.includes('disk')) return '#d29922'; // Yellow/Orange
    if (m.includes('net') || m.includes('bw')) return '#a371f7'; // Purple
    if (m.includes('err') || m.includes('fail')) return '#da3633'; // Red

    // Random fallback but deterministic
    let hash = 0;
    for (let i = 0; i < metric.length; i++) {
        hash = metric.charCodeAt(i) + ((hash << 5) - hash);
    }
    const c = (hash & 0x00FFFFFF).toString(16).toUpperCase();
    return '#' + "00000".substring(0, 6 - c.length) + c;
}

function hexToRgba(hex, alpha) {
    const r = parseInt(hex.slice(1, 3), 16);
    const g = parseInt(hex.slice(3, 5), 16);
    const b = parseInt(hex.slice(5, 7), 16);
    return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

function sanitizeId(str) {
    return str.replace(/[^a-zA-Z0-9]/g, '-');
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function formatTimeShort(dateStr) {
    const d = new Date(dateStr);
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function showToast(message, type = 'info') {
    const container = document.getElementById('toastContainer');
    const toast = document.createElement('div');

    toast.className = `toast ${type}`;
    toast.innerHTML = `
        <i class="fa-solid fa-${type === 'error' ? 'circle-exclamation' : type === 'success' ? 'check-circle' : 'info-circle'}"></i>
        <span>${escapeHtml(message)}</span>
    `;

    container.appendChild(toast);

    // Removal timer matches animation
    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transform = 'translateX(20px)'; // Match slideOut
        setTimeout(() => {
            if (toast.parentNode) container.removeChild(toast);
        }, 300);
    }, 3000);
}

function updateSetting(key, value) {
    appSettings[key] = value;
    localStorage.setItem('unostat_settings', JSON.stringify(appSettings));
    loadAllCharts(); // Redraw with new settings
}

async function clearAllData() {
    if (!confirm('Are you sure you want to delete ALL files and data? This cannot be undone.')) return;

    try {
        showToast('Clearing execution environment...', 'info');

        // Call the bulk delete API
        const response = await fetch('/api/files', { method: 'DELETE' });

        if (response.ok) {
            showToast('All data cleared', 'success');
            // Reset local state
            files = [];
            charts = {};
            loadFiles(); // Should return empty list
        } else {
            throw new Error('Server delete failed');
        }

    } catch (error) {
        console.error(error);
        showToast('Error clearing data', 'error');
    }
}

