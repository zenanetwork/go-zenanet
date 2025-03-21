# Builds a Windows installer with NSIS.
# It expects the following command line arguments:
# - OUTPUTFILE, filename of the installer (without extension)
# - MAJORVERSION, major build version
# - MINORVERSION, minor build version
# - BUILDVERSION, build id version
#
# The created installer executes the following steps:
# 1. install gzen for all users
# 2. install optional development tools such as abigen
# 3. create an uninstaller
# 4. configures the Windows firewall for gzen
# 5. create gzen, attach and uninstall start menu entries
# 6. configures the registry that allows Windows to manage the package through its platform tools
# 7. adds the environment system wide variable ZENANET_SOCKET
# 8. adds the install directory to %PATH%
#
# Requirements:
# - NSIS, http://nsis.sourceforge.net/Main_Page
# - NSIS Large Strings build, http://nsis.sourceforge.net/Special_Builds
# - SFP, http://nsis.sourceforge.net/NSIS_Simple_Firewall_Plugin (put dll in NSIS\Plugins\x86-ansi)
#
# After installing NSIS extra the NSIS Large Strings build zip and replace the makensis.exe and the
# files found in Stub.
#
# based on: http://nsis.sourceforge.net/A_simple_installer_with_start_menu_shortcut_and_uninstaller
#
# TODO:
# - sign installer
CRCCheck on

!define GROUPNAME "Zenanet"
!define APPNAME "Gzen"
!define DESCRIPTION "Official Go implementation of the Zenanet protocol"
!addplugindir .\

# Require admin rights on NT6+ (When UAC is turned on)
RequestExecutionLevel admin

# Use LZMA compression
SetCompressor /SOLID lzma

!include LogicLib.nsh
!include PathUpdate.nsh
!include EnvVarUpdate.nsh

!macro VerifyUserIsAdmin
UserInfo::GetAccountType
pop $0
${If} $0 != "admin" # Require admin rights on NT4+
  messageBox mb_iconstop "Administrator rights required!"
  setErrorLevel 740 # ERROR_ELEVATION_REQUIRED
  quit
${EndIf}
!macroend

function .onInit
  # make vars are global for all users since gzen is installed global
  setShellVarContext all
  !insertmacro VerifyUserIsAdmin

  ${If} ${ARCH} == "amd64"
    StrCpy $InstDir "$PROGRAMFILES64\${APPNAME}"
  ${Else}
    StrCpy $InstDir "$PROGRAMFILES32\${APPNAME}"
  ${Endif}
functionEnd

!include install.nsh
!include uninstall.nsh
