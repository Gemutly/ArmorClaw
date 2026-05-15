rule eicar_test_file {
    meta:
        description = "EICAR standard antivirus test file"
        severity = "high"
    strings:
        $eicar = "X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*" wide ascii nocase
    condition:
        $eicar
}

rule vba_auto_exec_macro {
    meta:
        description = "VBA auto-exec macro in Office documents"
        severity = "high"
    strings:
        $auto_open = "AutoOpen" nocase
        $auto_exec = "AutoExec" nocase
        $doc_open = "Document_Open" nocase
        $workbook_open = "Workbook_Open" nocase
    condition:
        any of them
}

rule vba_shell_execution {
    meta:
        description = "VBA macro with shell command execution"
        severity = "critical"
    strings:
        $shell = "Shell" nocase
        $wscript = "WScript.Shell" nocase
        $createobject = "CreateObject" nocase
        $run = ".Run" nocase ascii
    condition:
        $createobject and ($shell or $wscript) and $run
}

rule suspicious_javascript_download {
    meta:
        description = "JavaScript with suspicious download or eval patterns"
        severity = "high"
    strings:
        $eval = "eval(" nocase
        $fromcharcode = "fromCharCode" nocase
        $activex = "ActiveXObject" nocase
        $wscript_shell = "WScript.Shell" nocase
    condition:
        $eval and any of ($fromcharcode, $activex, $wscript_shell)
}

rule powershell_encoded_command {
    meta:
        description = "PowerShell with encoded command parameter"
        severity = "critical"
    strings:
        $enc = "-enc" nocase
        $encoded = "-encodedcommand" nocase
        $nop = "-noprofile" nocase
        $hidden = "-hidden" nocase
        $bypass = "-bypass" nocase
    condition:
        any of ($enc, $encoded) and any of ($nop, $hidden, $bypass)
}

rule powershell_web_request {
    meta:
        description = "PowerShell downloading content from the web"
        severity = "high"
    strings:
        $webclient = "System.Net.WebClient" nocase
        $downloadstring = "DownloadString" nocase
        $invoke_webrequest = "Invoke-WebRequest" nocase
        $iex = "iex" nocase
    condition:
        ($webclient and $downloadstring) or ($invoke_webrequest and $iex)
}

rule pe_header_in_non_pe {
    meta:
        description = "PE executable header embedded in non-PE file"
        severity = "critical"
    strings:
        $mz = "MZ" ascii
        $pe = "PE" ascii
        $this_program = { 4D 5A ?? ?? 50 45 00 00 }
    condition:
        $this_program
}

rule obfuscated_base64_script {
    meta:
        description = "Base64-encoded script content suggesting obfuscation"
        severity = "medium"
    strings:
        $powershell_b64 = "powershell" nocase
        $base64cmd = "-encodedcommand" nocase
        $long_b64 = /[A-Za-z0-9+\/]{200,}={0,2}/ ascii
    condition:
        $long_b64 and any of ($powershell_b64, $base64cmd)
}

rule exploit_kit_landing {
    meta:
        description = "Common exploit kit landing page patterns"
        severity = "critical"
    strings:
        $iframe_inject = /<iframe[^>]+src\s*=\s*['"]?(http|https):\/\// nocase
        $object_cls = /classid\s*=\s*['"]clsid:/ nocase
        $java_webkit = "java.lang.Runtime" nocase
        $pdf_exploit = /\/Annots\s/ ascii
    condition:
        ($iframe_inject and $object_cls) or $java_webkit or $pdf_exploit
}

rule macro_dropper_indicator {
    meta:
        description = "Macro-based document dropper pattern"
        severity = "high"
    strings:
        $chr_b64 = "Chr(" nocase
        $environ = "Environ" nocase
        $temp_path = "%TEMP%" nocase
        $appdata = "%APPDATA%" nocase
        $vba_shell = "Shell" nocase
    condition:
        $chr_b64 and $vba_shell and any of ($environ, $temp_path, $appdata)
}

rule embedded_script_in_archive {
    meta:
        description = "Script file embedded in archive or container"
        severity = "medium"
    strings:
        $vbs = ".vbs" nocase ascii
        $ps1 = ".ps1" nocase ascii
        $bat = ".bat" nocase ascii
        $cmd = ".cmd" nocase ascii
        $hta = ".hta" nocase ascii
        $hidden_ext = /"[^"]+\.(vbs|ps1|bat|cmd|hta)"/ nocase
    condition:
        any of them
}

rule certutil_download {
    meta:
        description = "CertUtil abused for file download"
        severity = "high"
    strings:
        $certutil = "certutil" nocase
        $urlcache = "-urlcache" nocase
        $verifyctl = "-verifyctl" nocase
    condition:
        $certutil and any of ($urlcache, $verifyctl)
}
