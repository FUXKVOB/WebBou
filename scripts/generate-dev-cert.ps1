param(
    [string]$CertPath = "cert.pem",
    [string]$KeyPath = "key.pem",
    [string]$CommonName = "localhost",
    [int]$DaysValid = 365
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Convert-ToPem {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Label,
        [Parameter(Mandatory = $true)]
        [byte[]]$Bytes
    )

    $base64 = [Convert]::ToBase64String($Bytes)
    $wrapped = ($base64 -split "(.{1,64})" | Where-Object { $_ }) -join "`n"
    "-----BEGIN $Label-----`n$wrapped`n-----END $Label-----`n"
}

function Export-PrivateKeyBytes {
    param(
        [Parameter(Mandatory = $true)]
        [System.Security.Cryptography.RSA]$Key
    )

    if ($Key.PSObject.Methods.Name -contains "ExportPkcs8PrivateKey") {
        return $Key.ExportPkcs8PrivateKey()
    }

    if ($Key -is [System.Security.Cryptography.RSACng]) {
        return $Key.Key.Export([System.Security.Cryptography.CngKeyBlobFormat]::Pkcs8PrivateBlob)
    }

    throw "This PowerShell/.NET runtime cannot export a PKCS#8 private key."
}

$notBefore = [DateTimeOffset]::UtcNow.AddMinutes(-5)
$notAfter = $notBefore.AddDays($DaysValid)
$outputDir = (Resolve-Path -LiteralPath ".").Path

$rsa = [System.Security.Cryptography.RSA]::Create(4096)
try {
    $subject = [System.Security.Cryptography.X509Certificates.X500DistinguishedName]::new("CN=$CommonName")
    $request = [System.Security.Cryptography.X509Certificates.CertificateRequest]::new(
        $subject,
        $rsa,
        [System.Security.Cryptography.HashAlgorithmName]::SHA256,
        [System.Security.Cryptography.RSASignaturePadding]::Pkcs1
    )

    $san = [System.Security.Cryptography.X509Certificates.SubjectAlternativeNameBuilder]::new()
    $san.AddDnsName($CommonName)
    if ($CommonName -ne "localhost") {
        $san.AddDnsName("localhost")
    }
    $san.AddIpAddress([System.Net.IPAddress]::Loopback)
    $san.AddIpAddress([System.Net.IPAddress]::IPv6Loopback)
    $request.CertificateExtensions.Add($san.Build())

    $request.CertificateExtensions.Add(
        [System.Security.Cryptography.X509Certificates.X509BasicConstraintsExtension]::new($false, $false, 0, $true)
    )
    $request.CertificateExtensions.Add(
        [System.Security.Cryptography.X509Certificates.X509KeyUsageExtension]::new(
            [System.Security.Cryptography.X509Certificates.X509KeyUsageFlags]::DigitalSignature -bor
            [System.Security.Cryptography.X509Certificates.X509KeyUsageFlags]::KeyEncipherment,
            $true
        )
    )

    $ekuOids = [System.Security.Cryptography.OidCollection]::new()
    $ekuOids.Add([System.Security.Cryptography.Oid]::new("1.3.6.1.5.5.7.3.1")) | Out-Null
    $eku = [System.Security.Cryptography.X509Certificates.X509EnhancedKeyUsageExtension]::new($ekuOids, $true)
    $request.CertificateExtensions.Add($eku)

    $certificate = $request.CreateSelfSigned($notBefore, $notAfter)
    try {
        $certPem = Convert-ToPem -Label "CERTIFICATE" -Bytes $certificate.Export([System.Security.Cryptography.X509Certificates.X509ContentType]::Cert)
        $keyPem = Convert-ToPem -Label "PRIVATE KEY" -Bytes (Export-PrivateKeyBytes -Key $rsa)

        [System.IO.File]::WriteAllText((Join-Path $outputDir $CertPath), $certPem, [System.Text.Encoding]::ASCII)
        [System.IO.File]::WriteAllText((Join-Path $outputDir $KeyPath), $keyPem, [System.Text.Encoding]::ASCII)

        Write-Host "Generated $CertPath and $KeyPath in $outputDir"
    }
    finally {
        $certificate.Dispose()
    }
}
finally {
    $rsa.Dispose()
}
