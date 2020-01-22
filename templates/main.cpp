#include <QApplication>
#include <QByteArray>
#include <QDir>
#include <QFile>
#include <QIODevice>
#include <QQmlEngine>
#include <QString>
#include <QTextStream>
#include <QUrl>
#include <QtDebug>
#include <QtGui/QCursor>
#include <QtGui/QFontDatabase>
#include <QtGui/QPixmap>
#include <QtQml/QQmlApplicationEngine>

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
    else
    {
        qWarning() << "Failed to open manifest";
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

    QQmlApplicationEngine engine;

    qmlRegisterSingletonType<QmlEnvironmentVariable>(
        "Hydra", 1, 0,
        "EnvironmentVariable",
        qmlenvironmentvariable_singletontype_provider);

    qmlRegisterSingletonType<QmlCursor>(
        "Hydra", 1, 0,
        "Cursor",
        qmlcursor_singletontype_provider);

    loadFonts();

    engine.load(QUrl(QStringLiteral("qrc:/app.qml")));

    app.exec();
}
