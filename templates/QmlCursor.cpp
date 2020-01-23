#include "QmlCursor.h"
#include <stdlib.h>

QApplication *QmlCursor::app = NULL;

void QmlCursor::push(const QString &resource)
{
    if (QmlCursor::app != NULL)
    {
        QmlCursor::app->setOverrideCursor(QCursor(QPixmap(resource)));
    }
}

void QmlCursor::pop()
{
    if (QmlCursor::app != NULL)
    {
        QmlCursor::app->restoreOverrideCursor();
    }
}

QObject *qmlcursor_singletontype_provider(QQmlEngine *, QJSEngine *)
{
    return new QmlCursor();
}