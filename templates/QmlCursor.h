#ifndef QMLCURSOR_H
#define QMLCURSOR_H

#include <QApplication>
#include <QObject>
#include <QtGui/QCursor>
#include <QtGui/QPixmap>

class QQmlEngine;
class QJSEngine;

class QmlCursor : public QObject
{
    Q_OBJECT
public:
    Q_INVOKABLE static void push(const QString &resource);
    Q_INVOKABLE static void pop();

    static QApplication *app;
};

// Define the singleton type provider function (callback).
QObject *qmlcursor_singletontype_provider(QQmlEngine*, QJSEngine*);

#endif // QMLCURSOR_H