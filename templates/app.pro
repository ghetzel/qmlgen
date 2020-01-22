DESTDIR = .
TARGET = catalina-ui

BUILDDIR = qt
OBJECTS_DIR = $${BUILDDIR}/obj
RESOURCES = app.qrc
MOC_DIR = $${BUILDDIR}/moc
RCC_DIR = $${BUILDDIR}/rcc
UI_DIR = $${BUILDDIR}/ui
MAKEFILE = $${BUILDDIR}/Makefile

SOURCES += \
    main.cpp \
    QmlCursor.cpp \
    QmlEnvironmentVariable.cpp

HEADERS += \
    QmlCursor.h \
    QmlEnvironmentVariable.h

QT += qml quick quickcontrols2 network widgets charts

static {
    QT += svg
    QTPLUGIN += qtvirtualkeyboardplugin
}

CONFIG -= app_bundle
CONFIG += c++11 disable-desktop
