#include <QApplication>
#include <QFile>
#include <QIODevice>
#include <QObject>
#include <QQmlApplicationEngine>
#include <QQmlComponent>
#include <QQmlContext>
#include <QString>
#include <QStringList>
#include <QTextStream>
#include <QtDebug>
#include <QtGui/QCursor>
#include <QtGui/QFontDatabase>

#include "QmlEnvironmentVariable.h"
#include "QmlCursor.h"

void loadFonts()
{
    QFile inputFile(QmlEnvironmentVariable::value("HYDRA_FONTS_MANIFEST", ":/styles/fonts/manifest.list"));

    if (inputFile.open(QIODevice::ReadOnly | QIODevice::Text))
    {
        QTextStream in(&inputFile);

        while (!in.atEnd())
        {
            QString line = in.readLine();

            line = line.trimmed();

            if (line.startsWith("#") || line.isEmpty())
            {
                continue;
            }

            qDebug() << "Loading font from manifest: " << line;

            if (QFontDatabase::addApplicationFont(line) < 0)
            {
                qDebug() << "Error loading " << line;
            }
        }

        inputFile.close();

        QFontDatabase db = QFontDatabase();
        QStringList fonts = db.families(QFontDatabase::Any);

        qDebug() << "Loaded fonts:";

        for (int i = 0; i < fonts.size(); ++i)
        {
            qDebug() << "  " << fonts.at(i);
        }
    }
}

int main(int argc, char **argv)
{
    QApplication app(argc, argv);
    QmlCursor::app = &app;

    QString cursorFile = QmlEnvironmentVariable::value("HYDRA_CURSOR", "");

    if (cursorFile != "")
    {
        app.setOverrideCursor(QCursor(QPixmap(cursorFile)));
    }

    qmlRegisterSingletonType<QmlEnvironmentVariable>(
        "Builtin", 1, 0,
        "EnvironmentVariable",
        qmlenvironmentvariable_singletontype_provider);

    qmlRegisterSingletonType<QmlCursor>(
        "Builtin", 1, 0,
        "Cursor",
        qmlcursor_singletontype_provider);

    loadFonts();

    QQmlApplicationEngine engine(
        QmlEnvironmentVariable::value("HYDRA_APP_QML", "qrc:/app.qml"));

    return app.exec();
}
