param([string]$BaseUrl = 'http://127.0.0.1:8080')

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Net.Http
$client = [System.Net.Http.HttpClient]::new()
$client.Timeout = [TimeSpan]::FromSeconds(10)

function Send-Request {
    param([string]$Method, [string]$Path, [int]$Expected, [string]$Body = '')
    $request = [System.Net.Http.HttpRequestMessage]::new([System.Net.Http.HttpMethod]::new($Method), "$BaseUrl$Path")
    if ($Body) {
        $request.Content = [System.Net.Http.StringContent]::new($Body, [System.Text.Encoding]::UTF8, 'application/json')
    }
    try {
        $response = $client.SendAsync($request).GetAwaiter().GetResult()
        try {
            $text = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
            $status = [int]$response.StatusCode
            Write-Host "`n$Method $Path -> $status $($response.ReasonPhrase)"
            if ($response.Headers.Location) { Write-Host "Location: $($response.Headers.Location)" }
            if ($text) { Write-Host $text.TrimEnd() }
            if ($status -ne $Expected) { throw "Expected HTTP $Expected, received $status" }
            if ($Expected -eq 204 -and $text.Length -ne 0) { throw 'DELETE response must have an empty body' }
            return $text
        } finally { $response.Dispose() }
    } finally { $request.Dispose() }
}

$resource = $null
try {
    Write-Host '=== DEVICE LIFECYCLE ==='
    $body = '{"name":"Main Thermal Sensor","ip_address":"192.168.1.50","type":"sensor","status":"online"}'
    $device = (Send-Request POST '/api/v1/devices' 201 $body) | ConvertFrom-Json
    $resource = "/api/v1/devices/$($device.id)"
    $null = Send-Request GET $resource 200
    $null = Send-Request GET '/api/v1/devices' 200
    $body = '{"name":"Main Thermal Sensor v2","ip_address":"192.168.1.51","type":"sensor","status":"offline"}'
    $null = Send-Request PUT $resource 200 $body
    $null = Send-Request DELETE $resource 204
    $null = Send-Request GET $resource 404
    $resource = $null
    Write-Host "`n=== VALIDATION AND MISSING RESOURCES ==="
    $null = Send-Request POST '/api/v1/devices' 400 '{"name":"Incomplete device"}'
    $null = Send-Request POST '/api/v1/devices' 400 '{invalid-json'
    $null = Send-Request POST '/api/v1/devices' 400 '{"name":"Sensor","ip_address":"bad-ip","type":"sensor"}'
    $null = Send-Request PUT '/api/v1/devices/unknown-device' 404 $body
    $null = Send-Request DELETE '/api/v1/devices/unknown-device' 404
    Write-Host "`n[PASS] CRUD lifecycle, validation and missing-resource checks completed"
} finally {
    try {
        if ($resource) { $null = Send-Request DELETE $resource 204 }
    } finally { $client.Dispose() }
}
