DESTDIR = .
TARGET = app

BUILDDIR = qt
OBJECTS_DIR = $${BUILDDIR}/obj
MOC_DIR = $${BUILDDIR}/moc
RCC_DIR = $${BUILDDIR}/rcc
UI_DIR = $${BUILDDIR}/ui
MAKEFILE = Makefile

RESOURCES += \
    app.qrc

SOURCES += \
    main.cpp \
    QmlCursor.cpp \
    QmlEnvironmentVariable.cpp

HEADERS += \
    QmlCursor.h \
    QmlEnvironmentVariable.h

QT += gui qml quick quickcontrols2 network widgets charts

static {
    QT += svg
    QTPLUGIN += qtvirtualkeyboardplugin
}

CONFIG += qtquickcompiler
CONFIG += c++11 disable-desktop
