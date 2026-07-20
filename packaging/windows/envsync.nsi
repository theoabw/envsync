Unicode true

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "WinMessages.nsh"

!ifndef VERSION
  !error "VERSION is required"
!endif
!ifndef PRODUCT_VERSION
  !error "PRODUCT_VERSION is required"
!endif
!ifndef ARCH
  !error "ARCH is required"
!endif
!ifndef INPUT_BINARY
  !error "INPUT_BINARY is required"
!endif
!ifndef OUTPUT_FILE
  !error "OUTPUT_FILE is required"
!endif

Name "envsync"
OutFile "${OUTPUT_FILE}"
InstallDir "$LOCALAPPDATA\Programs\envsync"
InstallDirRegKey HKCU "Software\envsync" "InstallDir"
RequestExecutionLevel user
SetCompressor /SOLID lzma

VIProductVersion "${PRODUCT_VERSION}"
VIAddVersionKey "ProductName" "envsync"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "FileDescription" "envsync installer (${ARCH})"
VIAddVersionKey "Publisher" "Theo Wilenius"
VIAddVersionKey "LegalCopyright" "Copyright Theo Wilenius"

!define MUI_ABORTWARNING
!define MUI_ICON "${NSISDIR}\Contrib\Graphics\Icons\modern-install.ico"
!define MUI_UNICON "${NSISDIR}\Contrib\Graphics\Icons\modern-uninstall.ico"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "${SOURCE_DIR}/LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_UNPAGE_FINISH

!insertmacro MUI_LANGUAGE "English"

Section "envsync" SecMain
  SectionIn RO
  SetOutPath "$INSTDIR"
  File "/oname=envsync.exe" "${INPUT_BINARY}"
  File "/oname=envsync-path.ps1" "${SOURCE_DIR}/packaging/windows/path.ps1"
  WriteUninstaller "$INSTDIR\uninstall.exe"

  WriteRegStr HKCU "Software\envsync" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\envsync" "DisplayName" "envsync"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\envsync" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\envsync" "Publisher" "Theo Wilenius"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\envsync" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\envsync" "DisplayIcon" "$INSTDIR\envsync.exe"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\envsync" "UninstallString" '$\"$INSTDIR\uninstall.exe$\"'
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\envsync" "NoModify" 1
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\envsync" "NoRepair" 1

  ExecWait 'powershell.exe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "$INSTDIR\envsync-path.ps1" add "$INSTDIR"' $0
  ${If} $0 != 0
    MessageBox MB_ICONEXCLAMATION "envsync was installed, but its directory could not be added to your PATH."
  ${EndIf}
  SendMessage ${HWND_BROADCAST} ${WM_SETTINGCHANGE} 0 "STR:Environment" /TIMEOUT=5000
SectionEnd

Section "Uninstall"
  ExecWait 'powershell.exe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "$INSTDIR\envsync-path.ps1" remove "$INSTDIR"' $0
  SendMessage ${HWND_BROADCAST} ${WM_SETTINGCHANGE} 0 "STR:Environment" /TIMEOUT=5000

  Delete "$INSTDIR\envsync.exe"
  Delete "$INSTDIR\envsync-path.ps1"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"

  DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\envsync"
  DeleteRegKey HKCU "Software\envsync"
SectionEnd
